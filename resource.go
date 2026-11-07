package llm

import (
	"bytes"
	"context"
	"io"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Resource is media, such as an image, document or audio, or a readable
// resource which a server advertises, which is opaque until it's read
type Resource interface {
	// URI returns the unique identifier of the resource. It must be an
	// absolute URI with a non-empty scheme (e.g. "file:///path/to/file").
	URI() string

	// Name returns a human-readable name for the resource.
	Name() string

	// Description returns an optional description of the resource.
	Description() string

	// Type returns the MIME type of the resource content, for example
	// image/png or application/pdf, or an empty string if unknown.
	Type() string

	// Read returns a function which reads the resource's content, or an
	// error when it can't be read, such as when it isn't found
	Read(context.Context) (ReadFunc, error)
}

// Sized is a resource whose content's length is known, without reading it,
// such as one which is kept in a store, so a provider can stream it, rather
// than reading it, to know how large it is
type Sized interface {
	// ContentLength returns the length of the content, in bytes, or -1, when
	// it isn't known
	ContentLength() int64
}

// ReadFunc reads a resource's content: it opens the content, calls the
// function with a reader of it, and closes it when the function returns.
// The reader returns an error once the context is cancelled. Returns the
// function's error, or the error opening or closing the content.
type ReadFunc func(context.Context, func(io.Reader) error) error

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ReadBytes returns a function which reads the data, for resources whose
// content is in memory
func ReadBytes(data []byte) ReadFunc {
	return func(ctx context.Context, fn func(io.Reader) error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return fn(bytes.NewReader(data))
	}
}

// ContentLength returns the length of the resource's content, in bytes,
// and true, when it's known without reading it, or false
func ContentLength(r Resource) (int64, bool) {
	if sized, ok := r.(Sized); ok {
		if n := sized.ContentLength(); n >= 0 {
			return n, true
		}
	}
	return 0, false
}

// ReadAll reads all of a resource's content, for providers which send it
// whole, such as encoded in a request
func ReadAll(ctx context.Context, r Resource) ([]byte, error) {
	read, err := r.Read(ctx)
	if err != nil {
		return nil, err
	}
	var data []byte
	if err := read(ctx, func(reader io.Reader) error {
		data, err = io.ReadAll(reader)
		return err
	}); err != nil {
		return nil, err
	}
	return data, nil
}
