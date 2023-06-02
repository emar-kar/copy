package copy

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// options allows to configure Copy behavior.
type options struct {
	force       bool
	contentOnly bool
}

type optFunc func(*options)

// Force re-write destination if it is already exists.
func Force(o *options) { o.force = true }

// ContentOnly if copy folder and set this flag to true,
// will copy only source content without creating root folder.
func ContentOnly(o *options) { o.contentOnly = true }

// Copy copies src to dst with given options.
func Copy(ctx context.Context, src, dst string, opts ...optFunc) error {
	opt := &options{}
	for _, fn := range opts {
		fn(opt)
	}

	src, err := evalSymlink(src)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot get source information: %w", err)
	}

	if srcInfo.IsDir() {
		if !opt.contentOnly {
			dst = path.Join(dst, path.Base(src))
		}

		return copyFolder(ctx, src, dst, opt.force)
	}

	if _, f := path.Split(src); f != path.Base(dst) {
		if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
			return err
		}

		dst = path.Join(dst, f)
	}

	if _, err := os.Stat(dst); !os.IsNotExist(err) || opt.force {
		return copyFile(ctx, src, dst)
	}

	return nil
}

// copyFolder is a support function to copy whole folder.
func copyFolder(ctx context.Context, src, dst string, force bool) error {
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
						return fmt.Errorf("cannot create sub-folder: %w", err)
					}

					return nil
				}

				if _, err := os.Stat(subDst); !os.IsNotExist(err) || force {
					return copyFile(ctx, root, subDst)
				}

				return nil
			}
		})
}

// copyFile is a support function to copy file content. Copies with buffer.
// If context canceled during the copy, dst file will be removed before return.
func copyFile(ctx context.Context, src, dst string) error {
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

// evalSymlink returns the path name after the evaluation of any symbolic links.
// Check [filepath.EvalSymlinks] for details.
func evalSymlink(p string) (string, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return "", fmt.Errorf("cannot get %s info: %w", p, err)
	}

	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		return filepath.EvalSymlinks(p)
	}

	return p, nil
}
