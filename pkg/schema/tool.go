package schema

import (
	"fmt"
	"net/url"

	// Packages
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

// BuiltinNamespace is the namespace used for locally-implemented (builtin) tools.
const BuiltinNamespace = "builtin"

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ToolListRequest represents a request to list tools
type ToolListRequest struct {
	pg.OffsetLimit

	// Namespace restricts results to a single namespace.
	// Use BuiltinNamespace for locally-implemented tools, a connector URL for
	// remote tools, or leave empty to include all namespaces.
	Namespace string `json:"namespace,omitempty"`

	// Name restricts results to tools whose names appear in this list.
	// An empty slice means no name filter — all names are included.
	Name []string `json:"name,omitempty"`
}

// ToolList represents a response containing a list of tools.
type ToolList struct {
	ToolListRequest
	Count uint       `json:"count"`
	Body  []ToolMeta `json:"body,omitzero"`
}

// ToolMeta represents a tool's metadata.
type ToolMeta struct {
	Name        string     `json:"name"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Input       JSONSchema `json:"input,omitempty"`
	Output      JSONSchema `json:"output,omitempty"`
	Hints       []string   `json:"hints,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r ToolMeta) String() string {
	return types.Stringify(r)
}

func (r ToolListRequest) String() string {
	return types.Stringify(r)
}

func (r ToolListRequest) Query() url.Values {
	values := url.Values{}
	if r.Offset > 0 {
		values.Set("offset", fmt.Sprintf("%d", r.Offset))
	}
	if r.Limit != nil {
		values.Set("limit", fmt.Sprintf("%d", types.Value(r.Limit)))
	}
	if r.Namespace != "" {
		values.Set("namespace", r.Namespace)
	}
	for _, name := range r.Name {
		if name != "" {
			values.Add("name", name)
		}
	}
	return values
}

func (r ToolList) String() string {
	return types.Stringify(r)
}
