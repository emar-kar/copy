package copy

// options allows to configure Copy behavior.
type options struct {
	bufSize     int
	force       bool
	contentOnly bool
	move        bool
}

func defaultOptions() *options {
	return &options{bufSize: 4096}
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
func WithBufferSize(size int) optFunc {
	return func(o *options) {
		o.bufSize = size
	}
}
