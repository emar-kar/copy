package copy

const defaultBufSize = 64 * 1024

type (
	// Option configures the behaviour of [Copy].
	Option func(*options)

	excludeFunc func(string) bool

	options struct {
		excludeFunc excludeFunc
		bufSize     int
		force       bool
		noFollow    bool
		into        bool
	}
)

func defaultOptions() *options {
	return &options{
		excludeFunc: func(_ string) bool { return false },
		bufSize:     defaultBufSize,
	}
}

// WithBufferSize sets the size of the in-memory buffer used when copying file
// contents. If size is <= 0 the default (64 KiB) is used.
func WithBufferSize(size int) Option {
	return func(o *options) {
		if size <= 0 {
			return
		}

		o.bufSize = size
	}
}

// WithExcludeFunc sets a predicate that controls which paths are copied.
// The function receives the absolute path of each entry; returning true skips
// that entry (and, for directories, their entire subtree).
func WithExcludeFunc(fn excludeFunc) Option {
	return func(o *options) {
		o.excludeFunc = fn
	}
}

// WithForce removes the destination before copying when it already exists,
// preventing an [fs.ErrExist] error.
func WithForce(o *options) { o.force = true }

// Deprecated: Use [WithForce] instead.
func Force(o *options) { WithForce(o) }

// WithNoFollow recreates symlinks encountered during a copy instead of
// following them to their targets. Broken symlinks are preserved as-is.
func WithNoFollow(o *options) { o.noFollow = true }

// Into copies src inside dst rather than directly to dst. The source's base
// name is appended to dst automatically. This is equivalent to adding a
// trailing path separator to dst but is more explicit and less error-prone.
//
//	copy.Copy(ctx, "src/dir", "dst/target", copy.Into)
//	// result: dst/target/dir/{files...}
func Into(o *options) { o.into = true }
