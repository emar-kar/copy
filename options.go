package copy

const defaultBufSize = 64 * 1024

type (
	optFunc     func(*options)
	excludeFunc func(string) bool

	options struct {
		excludeFunc excludeFunc
		bufSize     int
		force       bool
		noFollow    bool
	}
)

func defaultOptions() *options {
	return &options{
		excludeFunc: func(_ string) bool { return false },
		bufSize:     defaultBufSize,
	}
}

// WithBufferSize allows to set custom buffer size for file copy.
// If provided size is <= 0, then default will be used.
func WithBufferSize(i int) optFunc {
	return func(o *options) {
		if i <= 0 {
			return
		}

		o.bufSize = i
	}
}

// WithExcludeFunc allows to filter files and folders by path.
func WithExcludeFunc(fn excludeFunc) optFunc {
	return func(o *options) {
		o.excludeFunc = fn
	}
}

// Force rewrites destination if it already exists.
func Force(o *options) { o.force = true }

// WithNoFollow creates symlinks instead of resolving path and copying the contents.
func WithNoFollow(o *options) { o.noFollow = true }
