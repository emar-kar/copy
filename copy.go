// Package copy provides utilities for copying files and directory trees,
// with support for symlinks, context cancellation, and configurable options
// such as forced overwrites, exclude filters, and custom buffer sizes.
package copy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrSame is returned when the source and destination paths refer to the same
// file or directory, making the copy a no-op that would overwrite itself.
var ErrSame = errors.New("same location")

// Copy copies src to dst. Symlinks in src are resolved by default; pass
// [WithNoFollow] to recreate them as symlinks at the destination instead.
//
// If dst ends with a path separator (e.g. "dir/") or the [Into] option is set,
// src is copied into that directory under its base name. Intermediate
// directories are created as needed.
//
// If dst already exists, [fs.ErrExist] is returned unless the [WithForce] option
// is set, which removes the existing destination before copying. If src and dst
// resolve to the same inode, [ErrSame] is returned.
//
// Cancelling ctx stops the copy and surfaces the context error.
func Copy(ctx context.Context, src, dst string, opts ...Option) error {
	opt := defaultOptions()
	for _, fn := range opts {
		fn(opt)
	}

	lstat, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Make src absolute first so the exclude function always receives a clean,
	// rooted path that matches what copyTree passes for entries inside a tree.
	if src, err = filepath.Abs(src); err != nil {
		return err
	}

	// When Into is set, place src inside dst under its base name rather than
	// copying directly to dst. This is the programmatic equivalent of adding
	// a trailing path separator.
	if opt.into {
		dst = filepath.Join(dst, filepath.Base(src))
	}

	// Check the exclude predicate against the logical source path — i.e. the
	// path the caller specified — before any symlink resolution. This keeps the
	// exclude function's view consistent with the tree layout the user sees.
	if opt.excludeFunc(src) {
		return nil
	}

	link := lstat.Mode()&os.ModeSymlink != 0

	if link && opt.noFollow {
		// Recreate the symlink without resolving it. We still need the parent
		// directory, but we cannot call Stat on the (potentially broken) link
		// target, so use 0o755 for the parent unconditionally.
		dstDir, fileName := filepath.Split(dst)
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}

		if fileName == "" {
			dst = filepath.Join(dstDir, filepath.Base(src))
		}

		return os.Symlink(src, dst)
	}

	// From here on we are following symlinks, so resolve to the real target.
	if link {
		if src, err = filepath.EvalSymlinks(src); err != nil {
			return err
		}
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if dstInfo, err := os.Stat(dst); err == nil {
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

	dstDir, fileName := filepath.Split(dst)

	// Use the source directory's own permission bits when src is a directory.
	// When src is a plain file, always use 0o755 for the destination parent
	// directory.
	perm := srcInfo.Mode().Perm()
	if !srcInfo.IsDir() {
		perm = 0o755
	}

	if err := os.MkdirAll(dstDir, perm); err != nil {
		return err
	}

	if fileName == "" {
		dst = filepath.Join(dstDir, filepath.Base(src))
	}

	if srcInfo.IsDir() {
		return copyTree(ctx, src, dst, opt)
	}

	return copyFile(ctx, src, dst, opt)
}

// copyTree recursively copies the directory tree rooted at src into dst using
// [filepath.WalkDir]. Symlinks are resolved or recreated depending on opt.noFollow.
func copyTree(ctx context.Context, src, dst string, opt *options) error {
	return filepath.WalkDir(
		src, func(root string, d fs.DirEntry, err error) error {
			if err := ctx.Err(); err != nil {
				return err
			}

			if err != nil {
				return err
			}

			link := d.Type()&fs.ModeSymlink != 0

			// Pass the original walk path (root) to the exclude predicate, not the
			// resolved symlink target. The resolved target may be in a completely
			// different directory, so an exclude rule based on tree location would
			// never match if we used res instead.
			if opt.excludeFunc(root) {
				// Return fs.SkipDir for excluded directories so WalkDir skips the
				// entire subtree. Returning nil would still descend into children,
				// visiting each one individually even though the whole directory is
				// meant to be filtered out.
				if d.IsDir() {
					return fs.SkipDir
				}

				return nil
			}

			res := root
			if link {
				if res, err = filepath.EvalSymlinks(root); err != nil {
					return err
				}
			}

			rel, err := filepath.Rel(src, root)
			if err != nil {
				return err
			}

			subDst := filepath.Join(dst, rel)

			if link && opt.noFollow {
				return os.Symlink(res, subDst)
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			if link {
				if info, err = os.Stat(res); err != nil {
					return err
				}
			}

			if info.IsDir() {
				return os.MkdirAll(subDst, info.Mode())
			}

			// Always use 0o755 for the parent directory of a file entry.
			// info.Mode() is the file's permission and may lack the execute bit
			// (e.g. 0o644), which would make the created directory unenterable.
			if err := os.MkdirAll(filepath.Dir(subDst), 0o755); err != nil {
				return err
			}

			target := root
			if link {
				target = res
			}

			return copyFile(ctx, target, subDst, opt)
		},
	)
}

// copyFile copies the regular file at src to dst, preserving its permission
// bits. The caller must ensure dst's parent directory already exists.
func copyFile(ctx context.Context, src, dst string, opt *options) (err error) {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	stat, err := srcF.Stat()
	if err != nil {
		return err
	}

	dstF, err := os.OpenFile(
		dst,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		stat.Mode().Perm(),
	)
	if err != nil {
		return err
	}
	defer dstF.Close()

	return copyBytes(ctx, srcF, dstF, opt.bufSize)
}

// copyBytes reads from r and writes to w in chunks of the given buffer size
// until EOF or the context is cancelled. Context is checked before each read
// to allow prompt cancellation.
func copyBytes(ctx context.Context, r io.Reader, w io.Writer, size int) error {
	buf := make([]byte, size)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		b, err := r.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		if b == 0 {
			return nil
		}

		if _, err := w.Write(buf[:b]); err != nil {
			return err
		}
	}
}
