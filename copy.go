package copy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/emar-kar/copy/v2/internal/contextio"
	"github.com/emar-kar/copy/v2/internal/utils"
)

var ErrSame = errors.New("same location")

func Copy(ctx context.Context, src, dst string, opts ...optFunc) error {
	opt := defaultOptions()
	for _, fn := range opts {
		fn(opt)
	}

	var (
		link bool
		err  error
	)

	src, link, err = utils.ResolvePath(src)
	if err != nil {
		return err
	}

	if opt.excludeFunc(src) {
		return nil
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if dstInfo, err := os.Stat(dst); !errors.Is(err, fs.ErrNotExist) {
		if os.SameFile(srcInfo, dstInfo) {
			return ErrSame
		}

		if !opt.force {
			return fmt.Errorf("%s: %w", dst, fs.ErrExist)
		}

		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}

	dstDir, fileName := path.Split(dst)

	if err := os.MkdirAll(dstDir, srcInfo.Mode()); err != nil {
		return err
	}

	if fileName == "" {
		dst = path.Join(dstDir, path.Base(src))
	}

	if link && opt.noFollow {
		return os.Symlink(src, dst)
	}

	if srcInfo.IsDir() {
		return copyTree(ctx, src, dst, opt)
	}

	return copyFile(ctx, src, dst, opt)
}

func copyTree(ctx context.Context, src, dst string, opt *options) error {
	return filepath.Walk(
		src, func(root string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			res, link, err := utils.ResolvePath(root)
			if err != nil {
				return err
			}

			if opt.excludeFunc(res) {
				return nil
			}

			if link {
				info, err = os.Stat(res)
				if err != nil {
					return err
				}
			}

			subDst := strings.ReplaceAll(root, src, dst)

			switch {
			case link && opt.noFollow:
				return os.Symlink(res, subDst)
			case info.IsDir():
				return os.MkdirAll(subDst, info.Mode())
			default:
				if err := os.MkdirAll(path.Dir(subDst), info.Mode()); err != nil {
					return err
				}

				return copyFile(ctx, root, subDst, opt)
			}
		},
	)
}

func copyFile(ctx context.Context, src, dst string, opt *options) (err error) {
	stat, err := os.Stat(src)
	if err != nil {
		return err
	}

	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.OpenFile(
		dst,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC,
		stat.Mode().Perm(),
	)
	if err != nil {
		return err
	}
	defer dstF.Close()

	return copyBytes(ctx, srcF, dstF, opt.bufSize)
}

func copyBytes(ctx context.Context, r io.Reader, w io.Writer, size int) error {
	src := contextio.Reader(ctx, r)
	dst := contextio.Writer(ctx, w)
	buf := make([]byte, size)

	for {
		b, err := src.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		if b == 0 {
			return nil
		}

		if _, err := dst.Write(buf[:b]); err != nil {
			return err
		}
	}
}
