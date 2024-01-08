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
	opt := defaultOptions()
	for _, fn := range opts {
		fn(opt)
	}

	if opt.move {
		info, err := os.Stat(src)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			srcDir, _ := path.Split(src)
			info, err = os.Stat(srcDir)
			if err != nil {
				return err
			}
		}

		dstDir, _ := path.Split(dst)
		if err := os.MkdirAll(dstDir, info.Mode()); err != nil {
			return err
		}

		if err := os.Rename(src, dst); err == nil {
			return os.RemoveAll(src)
		}
	}

	return copy(ctx, src, dst, opt)
}

func copy(ctx context.Context, src, dst string, opt *options) error {
	src, err := resolvePath(src)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		if !opt.contentOnly {
			dst = path.Join(dst, path.Base(src))
		}

		return copyFolder(ctx, src, dst, opt)
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

	if _, err := os.Stat(dst); os.IsNotExist(err) || opt.force {
		return copyFile(ctx, src, dst, opt)
	}

	if opt.move {
		return os.RemoveAll(src)
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
func copyFile(ctx context.Context, src, dst string, opt *options) error {
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

	buf := make([]byte, opt.bufSize)

	srcReader := NewReadWriterWithContext(ctx, srcF)
	dstWriter := NewReadWriterWithContext(ctx, dstF)

	for {
		b, err := srcReader.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		if b == 0 {
			return nil
		}

		if _, err := dstWriter.Write(buf[:b]); err != nil {
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

// ReadWriterWithContext wraps [io.ReadWriter] and adds [context.Context]
// to its Read and Write methods, so those operations can be canceled.
type ReadWriterWithContext struct {
	ctx context.Context
	rw  io.ReadWriter
}

func (rw *ReadWriterWithContext) Read(b []byte) (int, error) {
	if err := rw.ctx.Err(); err != nil {
		return 0, err
	}

	return rw.rw.Read(b)
}

func (rw *ReadWriterWithContext) Write(b []byte) (int, error) {
	if err := rw.ctx.Err(); err != nil {
		return 0, err
	}

	return rw.rw.Write(b)
}

func NewReadWriterWithContext(
	ctx context.Context, rw io.ReadWriter,
) *ReadWriterWithContext {
	return &ReadWriterWithContext{ctx, rw}
}
