package utils

import (
	"os"
	"path/filepath"
)

// ResolvePath resolves symlinks and relative paths.
func ResolvePath(p string) (string, bool, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return "", false, err
	}

	var isLink bool

	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		isLink = true

		if p, err = filepath.EvalSymlinks(p); err != nil {
			return "", isLink, err
		}
	}

	p, err = filepath.Abs(p)

	return p, isLink, err
}
