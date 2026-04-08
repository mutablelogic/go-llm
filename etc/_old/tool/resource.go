package tool

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// JSONResource wraps raw JSON bytes as an llm.Resource with MIME type
// application/json. Use this to return JSON output from tool Run methods.
type JSONResource struct {
	data []byte
}

var _ llm.Resource = (*JSONResource)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewJSONResource creates a Resource wrapping the given JSON bytes.
func NewJSONResource(data []byte) *JSONResource {
	return &JSONResource{data: data}
}

///////////////////////////////////////////////////////////////////////////////
// RESOURCE INTERFACE

func (r *JSONResource) URI() string         { return "data:application/json" }
func (r *JSONResource) Name() string        { return "" }
func (r *JSONResource) Description() string { return "" }
func (r *JSONResource) Type() string        { return "application/json" }

func (r *JSONResource) Read(_ context.Context) ([]byte, error) {
	return r.data, nil
}
