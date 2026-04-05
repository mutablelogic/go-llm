package schema

import (
	"encoding/json"
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
	Namespace string `json:"namespace,omitempty" help:"Restrict results to a single namespace" example:"builtin"`

	// Name restricts results to tools whose names appear in this list.
	// An empty slice means no name filter — all names are included.
	Name []string `json:"name,omitempty" help:"Restrict results to the listed tool names" example:"[\"builtin.search_docs\",\"builtin.fetch_url\"]"`
}

// ToolList represents a response containing a list of tools.
type ToolList struct {
	ToolListRequest
	Count uint       `json:"count" help:"Total number of matching tools" example:"2"`
	Body  []ToolMeta `json:"body,omitzero" help:"Tool metadata returned for the current page" example:"[{\"name\":\"builtin.search_docs\",\"title\":\"Search Docs\"}]"`
}

// ToolMeta represents a tool's metadata.
type ToolMeta struct {
	Name        string     `json:"name" help:"Fully-qualified tool name" example:"builtin.search_docs"`
	Title       string     `json:"title,omitempty" help:"Human-readable tool title" example:"Search Docs"`
	Description string     `json:"description,omitempty" help:"Short description of what the tool does" example:"Search project documentation by keyword."`
	Input       JSONSchema `json:"input,omitempty" help:"JSON schema describing the tool input" example:"{\"type\":\"object\",\"properties\":{\"query\":{\"type\":\"string\"}}}"`
	Output      JSONSchema `json:"output,omitempty" help:"JSON schema describing the tool output" example:"{\"type\":\"object\",\"properties\":{\"results\":{\"type\":\"array\"}}}"`
	Hints       []string   `json:"hints,omitempty" help:"Additional usage hints for the tool" example:"[\"read_only\"]"`
}

// CallToolRequest represents a request to call a tool directly.
type CallToolRequest struct {
	Input json.RawMessage `json:"input,omitempty" help:"JSON-encoded arguments passed to the tool" example:"{\"query\":\"authentication flow\"}"`
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

func (r CallToolRequest) String() string {
	return types.Stringify(r)
}
