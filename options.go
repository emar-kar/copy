package copy

// options allows to configure Copy behavior.
type options struct {
	force       bool
	contentOnly bool
	move        bool
}

type optFunc func(*options)

// Force re-writes destination if it is already exists.
func Force(o *options) { o.force = true }

// ContentOnly copies only source folder content without creating
// root folder in destination.
func ContentOnly(o *options) { o.contentOnly = true }

// WithMove removes src after copying process is finished.
func WithMove(o *options) { o.move = true }
