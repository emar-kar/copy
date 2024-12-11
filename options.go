package copy

import "hash"

const defaultBufferSize = 4096

// options allows to configure Copy behavior.
type options struct {
	exclude     []string
	hash        hash.Hash
	bufSize     int
	force       bool
	contentOnly bool
	move        bool
	revert      bool
	follow      bool
}

func defaultOptions() *options {
	return &options{bufSize: defaultBufferSize}
}

type (
	optFunc func(*options)

	// Type to create custom slices of copy options.
	Options []optFunc
)

// Force re-writes destination if it is already exists.
func Force(o *options) { o.force = true }

// ContentOnly copies only source folder content without creating
// root folder in destination.
func ContentOnly(o *options) { o.contentOnly = true }

// FollowSymlink resolves source file path before copy, to follow symlink
// if needed.
func FollowSymlink(o *options) { o.follow = true }

// WithMove removes source after copying process is finished.
func WithMove(o *options) { o.move = true }

// WithBufferSize allows to set custom buffer size for file copy.
// If provided size <= 0, then default will be used.
func WithBufferSize(size int) optFunc {
	return func(o *options) {
		if size <= 0 {
			return
		}

		o.bufSize = size
	}
}

// RevertOnErr removes destination file if there was an error during copy process.
func RevertOnErr(o *options) { o.revert = true }

// WithHash calculates hash of the copied file(s).
//
// Note: if hash is not nil, it guarantee the copied file(s) will be read.
// Might increase total execution time.
func WithHash(h hash.Hash) optFunc {
	return func(o *options) {
		o.hash = h
	}
}

// WithExclude excludes paths from copy which includes one of the given
// strings.
func WithExclude(s ...string) optFunc {
	return func(o *options) {
		o.exclude = append(o.exclude, s...)
	}
}
