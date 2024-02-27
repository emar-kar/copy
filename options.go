package copy

import "hash"

const defaultBufferSize = 4096

// options allows to configure Copy behavior.
type options struct {
	hash        hash.Hash
	bufSize     int
	force       bool
	contentOnly bool
	move        bool
	revert      bool
}

func defaultOptions() *options {
	return &options{bufSize: defaultBufferSize}
}

type optFunc func(*options)

// Force re-writes destination if it is already exists.
func Force(o *options) { o.force = true }

// ContentOnly copies only source folder content without creating
// root folder in destination.
func ContentOnly(o *options) { o.contentOnly = true }

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
