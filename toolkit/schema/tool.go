package schema

import (
	"encoding/json"
	"net/url"
	"strings"

	// Packages
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Tool is a tool which models can call
type Tool struct {
	Name        string          `json:"name" readonly:"" help:"Tool name" example:"current"`
	Canonical   string          `json:"canonical" readonly:"" help:"Namespace and tool name, separated by a colon, which refers to the tool unambiguously" example:"weatherapi:current"`
	Description string          `json:"description,omitempty" readonly:"" help:"Description of the tool" example:"Get current weather conditions for a location"`
	Input       json.RawMessage `json:"input,omitempty" readonly:"" help:"JSON schema of the tool's input"`
	Output      json.RawMessage `json:"output,omitempty" readonly:"" help:"JSON schema of the tool's output, if it has one"`
}

// ToolListRequest filters a list of tools
type ToolListRequest struct {
	Namespace string `json:"namespace,omitempty" help:"Returns only the tools in this namespace" example:"weatherapi" optional:""`
}

// ToolList is the tools, with their number
type ToolList struct {
	ToolListRequest
	Count uint64  `json:"count"`
	Body  []*Tool `json:"body,omitempty"`
}

// CallToolRequest is a request to run a tool with its input
type CallToolRequest struct {
	Input json.RawMessage `json:"input,omitempty" help:"Input to the tool, which must match its input schema" example:"{\"query\":\"London\"}" optional:""`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// ToolSeparator separates the namespace and name in a tool's canonical
	// name
	ToolSeparator = ":"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CanonicalToolName returns the canonical name of a tool in a namespace,
// which is the namespace and tool name separated by a colon
func CanonicalToolName(namespace, name string) string {
	return namespace + ToolSeparator + name
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (t Tool) String() string {
	return types.Stringify(t)
}

func (t ToolListRequest) String() string {
	return types.Stringify(t)
}

func (t ToolList) String() string {
	return types.Stringify(t)
}

func (t CallToolRequest) String() string {
	return types.Stringify(t)
}

////////////////////////////////////////////////////////////////////////////////
// TABLE OUTPUT

func (t Tool) Header() []string {
	return []string{"Name", "Description"}
}

func (t Tool) Width(col int) int {
	return 0
}

func (t Tool) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return t.Canonical
	case 1:
		return t.Description
	default:
		return ""
	}
}

////////////////////////////////////////////////////////////////////////////////
// QUERY

// Query returns the request as URL query parameters
func (t ToolListRequest) Query() url.Values {
	q := url.Values{}
	if namespace := strings.TrimSpace(t.Namespace); namespace != "" {
		q.Set("namespace", namespace)
	}
	return q
}
