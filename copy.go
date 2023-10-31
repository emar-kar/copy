package copy

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Copy copies src to dst with given options.
func Copy(ctx context.Context, src, dst string, opt ...optFunc) error {
	opts := &options{}
	for _, fn := range opt {
		fn(opts)
	}

	if err := copy(ctx, src, dst, opts); err != nil {
		if opts.skip {
			if opts.log {
				log.Println(err)
			}

			return nil
		}

		return err
	}

	return nil
}

func copy(ctx context.Context, src, dst string, opts *options) error {
	src, err := resolvePath(src)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot get source information: %w", err)
	}

	if srcInfo.IsDir() {
		if !opts.contentOnly {
			dst = path.Join(dst, path.Base(src))
		}

		return copyFolder(ctx, src, dst, opts)
	}

	if _, f := path.Split(src); f != path.Base(dst) {
		if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
			return err
		}

		dst = path.Join(dst, f)
	}

	if _, err := os.Stat(dst); !os.IsNotExist(err) || opts.force {
		return copyFile(ctx, src, dst, opts)
	}

	return nil
}

// copyFolder is a support function to copy whole folder.
func copyFolder(ctx context.Context, src, dst string, opts *options) error {
	return filepath.Walk(
		src, func(root string, info fs.FileInfo, err error,
		) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err != nil {
					return err
				}

				subDst := strings.ReplaceAll(root, src, dst)
				if info.IsDir() {
					if err := os.MkdirAll(subDst, info.Mode()); err != nil {
						return err
					}
				}

				if _, err := os.Stat(subDst); !os.IsNotExist(err) || opts.force {
					return copyFile(ctx, root, subDst, opts)
				}

				return nil
			}
		})
}

// copyFile is a support function to copy file content. Copies with buffer.
// If context canceled during the copy, dst file will be removed before return.
func copyFile(ctx context.Context, src, dst string, opts *options) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	stat, err := os.Stat(src)
	if err != nil {
		return err
	}

	dstF, err := os.OpenFile(
		dst,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC,
		stat.Mode().Perm(),
	)
	if err != nil {
		return err
	}
	defer dstF.Close()

	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			if err := os.RemoveAll(dst); err != nil {
				return fmt.Errorf(
					"%w: cannot remove dst file: %w", ctx.Err(), err,
				)
			}

			return ctx.Err()
		default:
			b, err := srcF.Read(buf)
			if err != nil && err != io.EOF {
				return err
			}

			if b == 0 {
				return nil
			}

			if _, err := dstF.Write(buf[:b]); err != nil {
				return err
			}
		}
	}
}

// resolvePath resolves symlinks and relative paths.
func resolvePath(p string) (string, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return "", err
	}

	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		if p, err = filepath.EvalSymlinks(p); err != nil {
			return "", err
		}
	}

	return filepath.Abs(p)
}
