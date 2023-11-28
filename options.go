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
// Note: this flag silence all possible copy errors. It
// does not check if file/folder was actually copied.
func WithErrorsSkip(o *options) { o.skip = true }

// WithErrorsLog logs errors during find execution,
// should be used with [WithErrorsSkip], for clear output.
func WithErrorsLog(o *options) { o.log = true }
