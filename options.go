package copy

// options allows to configure Copy behavior.
type options struct {
	force       bool
	contentOnly bool
	skip        bool
	log         bool
}

type optFunc func(*options)

// Force re-write destination if it is already exists.
func Force(o *options) { o.force = true }

// ContentOnly if copy folder and set this flag to true,
// will copy only source content without creating root folder.
func ContentOnly(o *options) { o.contentOnly = true }

// WithErrorsSkip skips errors during find execution.
//
// Note: if the flag was set, [Copy] will return nil error,
// only if the base path was resolved and exists and if it was
// possible to create destination file or folder.
func WithErrorsSkip(o *options) { o.skip = true }

// WithErrosLog logs errors during find execution,
// should be used with [WithErrorsSkip], for clear output.
func WithErrosLog(o *options) { o.log = true }
