package copy

import (
	"context"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Copy copies source file/folder to destination with given options.
func Copy(ctx context.Context, src, dst string, opts ...optFunc) (err error) {
	opt := defaultOptions()
	for _, fn := range opts {
		fn(opt)
	}

	if opt.follow {
		src, err = resolvePath(src)
		if err != nil {
			return err
		}
	}
	}

	// Attempt to rename file/folder instead of copying and then removing.
	// If call to rename was finished with an error, it will be ignored
	// and copy algorithm will be used. In case of renaming a folder,
	// hash is not calculated and will be empty.
	if opt.move {
		if err := rename(src, dst, opt.hash); err == nil {
			return os.RemoveAll(src)
		}

		if opt.hash != nil {
			// Reset hash if rename was unsuccessful, since it will be
			// recalculated with copy if needed.
			opt.hash.Reset()
		}
	}

	return copy(ctx, src, dst, opt)
}

func rename(src, dst string, h hash.Hash) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		if h != nil {
			f, err := os.Open(src)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(h, f); err != nil {
				return err
			}
		}

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

	dstDir, f := path.Split(dst)
	if f == "" {
		f = path.Base(src)
	}

	info, err := os.Stat(path.Dir(src))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dstDir, info.Mode()); err != nil {
		return err
	}

	dst = path.Join(dstDir, f)

	if _, err := os.Stat(dst); os.IsNotExist(err) || opt.force {
		return copyFile(ctx, src, dst, opt)
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

	var mw io.Writer

	if opt.hash != nil {
		mw = io.MultiWriter(dstF, opt.hash)
	} else {
		mw = dstF
	}

	if cErr := copyBytes(ctx, srcF, mw, opt.bufSize, opt.hash); cErr != nil {
		if opt.revert {
			if rErr := os.Remove(dst); rErr != nil {
				return fmt.Errorf("%w: %s", cErr, rErr)
			}
		}

		return cErr
	}

	if opt.move {
		return os.Remove(src)
	}

	return nil
}

// copyBytes is a support function to copy bytes from [io.Reader] to [io.Writer] with given
// size buffer and hash.
func copyBytes(ctx context.Context, r io.Reader, w io.Writer, size int, h hash.Hash) error {
	buf := make([]byte, size)

	if h != nil {
		w = io.MultiWriter(w, h)
	}

	srcReader := &readerWithContext{ctx, r}
	dstWriter := &writerWithContext{ctx, w}

	for {
		b, err := srcReader.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
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

type readerWithContext struct {
	ctx context.Context
	r   io.Reader
}

func (r *readerWithContext) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	return r.r.Read(b)
}

type writerWithContext struct {
	ctx context.Context
	w   io.Writer
}

func (w *writerWithContext) Write(b []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}

	return w.w.Write(b)
}
