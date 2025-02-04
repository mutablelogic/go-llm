package openai

import (
	// Packages
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embeddings: The number of dimensions the resulting output embeddings
// should have. Only supported in text-embedding-3 and later models.
func WithDimensions(v uint64) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("dimensions", v)
		return nil
	}
}

// A unique identifier representing your end-user
func WithUser(v string) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("user", v)
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func optFormat(opts *llm.Opts) string {
	return opts.GetString("format")
}

func optDimensions(opts *llm.Opts) uint64 {
	return opts.GetUint64("dimensions")
}

func optUser(opts *llm.Opts) string {
	return opts.GetString("user")
}
