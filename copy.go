package copy

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Copy copies src to dst with given options.
func Copy(ctx context.Context, src, dst string, opts ...optFunc) error {
	opt := &options{}
	for _, fn := range opts {
		fn(opt)
	}

	if err := copy(ctx, src, dst, opt); err != nil {
		return err
	}

	if opt.move {
		return os.RemoveAll(src)
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
		return err
	}

	if srcInfo.IsDir() {
		if !opts.contentOnly {
			dst = path.Join(dst, path.Base(src))
		}

		return copyFolder(ctx, src, dst, opts)
	}

	if dir, f := path.Split(src); f != path.Base(dst) {
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}

		dst = path.Join(dst, f)
	}

	if _, err := os.Stat(dst); os.IsNotExist(err) || opts.force {
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
				} else {
					if _, err := os.Stat(subDst); os.IsNotExist(err) || opts.force {
						return copyFile(ctx, root, subDst, opts)
					}
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
