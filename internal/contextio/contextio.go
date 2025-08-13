package contextio

import (
	"context"
	"io"
)

type reader struct {
	ctx context.Context
	r   io.Reader
}

// Reader returns [io.Reader] implementation wrapped with [context.Context].
// Its Read method returns a context error on call if it is not nil.
func Reader(ctx context.Context, r io.Reader) *reader { return &reader{ctx, r} }

func (r *reader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	return r.r.Read(p)
}

type writer struct {
	ctx context.Context
	w   io.Writer
}

// Writer returns [io.Writer] implementation wrapped with [context.Context].
// Its Write method returns a context error on call if it is not nil.
func Writer(ctx context.Context, w io.Writer) *writer { return &writer{ctx, w} }

func (w *writer) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}

	return w.w.Write(p)
}
