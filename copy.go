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

// Copy copies source file/folder to destination with given options.
func Copy(ctx context.Context, src, dst string, opts ...optFunc) error {
	opt := defaultOptions()
	for _, fn := range opts {
		fn(opt)
	}

	src, err := resolvePath(src)
	if err != nil {
		return err
	}

	// Attempt to rename file/folder instead of copying and then removing.
	// If call to rename was finished with an error, it will be ignored
	// and standart copy algorithm will be used.
	if opt.move {
		if err := rename(src, dst); err == nil {
			return os.RemoveAll(src)
		}
	}

	return copy(ctx, src, dst, opt)
}

func rename(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		info, err = os.Stat(filepath.Dir(src))
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(dst), info.Mode()); err != nil {
		return err
	}

	return os.Rename(src, dst)
}

func copy(ctx context.Context, src, dst string, opt *options) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
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
		if err := copyFile(ctx, src, dst, opt); err != nil {
			return err
		}
	}

	return nil
}

// copyFolder is a support function to copy folder and its' content.
func copyFolder(ctx context.Context, src, dst string, opt *options) error {
	if !opt.contentOnly {
		dst = path.Join(dst, path.Base(src))
	}

	if err := filepath.Walk(
		src, func(root string, info fs.FileInfo, err error,
		) error {
			if err != nil {
				return err
			}

			subDst := strings.ReplaceAll(root, src, dst)
			if info.IsDir() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					return os.MkdirAll(subDst, info.Mode())
				}
			} else {
				if _, err := os.Stat(subDst); os.IsNotExist(err) || opt.force {
					return copyFile(ctx, root, subDst, opt)
				}
			}

			return nil
		}); err != nil {
		return err
	}

	if opt.move {
		return os.RemoveAll(src)
	}

	return nil
}

// copyFile is a support function to copy file content.
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

	if cErr := copyBytes(ctx, srcF, dstF, opt.bufSize); cErr != nil && opt.revert {
		if rErr := os.Remove(dst); rErr != nil {
			return fmt.Errorf("%w: %s", cErr, rErr)
		}

		return cErr
	}

	if opt.move {
		return os.Remove(src)
	}

	return nil
}

// copyBytes is a support function to copy bytes from one [os.File]
// to another with the given size buffer.
func copyBytes(ctx context.Context, r, w *os.File, size int) error {
	buf := make([]byte, size)

	srcReader := NewReadWriterWithContext(ctx, r)
	dstWriter := NewReadWriterWithContext(ctx, w)

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
