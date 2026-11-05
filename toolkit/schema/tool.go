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

// LogEvent is the data of a log event, of a line which a tool logs, as it
// runs, when it's called with a stream
type LogEvent struct {
	Text string `json:"text" help:"Line which the tool logged"`
}

// ProgressEvent is the data of a progress event, of how a tool is getting
// on, as it runs, when it's called with a stream, as MCP's progress
// notification is, with its status and percent, as a model's download is
type ProgressEvent struct {
	Status   string  `json:"status" help:"What the tool is doing, which is its message"`
	Percent  float64 `json:"percent" help:"Percentage which is complete, from 0 to 100, or zero when it isn't known"`
	Progress float64 `json:"progress,omitempty" help:"How far the tool has got, which increases each time"`
	Total    float64 `json:"total,omitempty" help:"How far the tool has to go, when it's known"`
	Message  string  `json:"message,omitempty" help:"What the tool is doing"`
}

// ErrorEvent is the data of an error event, which ends a stream when the
// tool fails
type ErrorEvent struct {
	Error string `json:"error" help:"Error of the tool, as it would be without a stream"`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// ToolSeparator separates the namespace and name in a tool's canonical
	// name
	ToolSeparator = ":"

	// The events of a stream of a tool which is called: a log event for
	// each line it logs, a progress event for how it's getting on, and then
	// a result event, with its result, or an error event
	EventLog      = "log"
	EventProgress = "progress"
	EventResult   = "result"
	EventError    = "error"

	// ModelToolSeparator separates the namespace and name in the name a
	// model calls a tool by, which a namespace, or a tool's name, can't
	// have, so it's separated where it is
	ModelToolSeparator = "__"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CanonicalToolName returns the canonical name of a tool in a namespace,
// which is the namespace and tool name separated by a colon
func CanonicalToolName(namespace, name string) string {
	return namespace + ToolSeparator + name
}

// ModelToolName returns the name a model calls a tool in a namespace by,
// which is the namespace and tool name separated by two underscores, as a
// model's tool names can't have a colon, such as homeassistant__call_service
func ModelToolName(namespace, name string) string {
	return namespace + ModelToolSeparator + name
}

// CanonicalFromModelToolName returns the canonical name of the tool which a
// model calls by the name, such as homeassistant:call_service for
// homeassistant__call_service, or the name, when it isn't a tool in a
// namespace, such as respond
func CanonicalFromModelToolName(name string) string {
	if namespace, tool, ok := strings.Cut(name, ModelToolSeparator); ok && namespace != "" && tool != "" {
		return CanonicalToolName(namespace, tool)
	}
	return name
}

// IsModelToolPart returns true when the name, of a namespace, or of a tool
// in one, can be part of the name a model calls a tool by, so it's
// separated where it is: it doesn't have two underscores, and doesn't start,
// or end, with one, which would be next to the separator
func IsModelToolPart(name string) bool {
	return !strings.Contains(name, ModelToolSeparator) && !strings.HasPrefix(name, "_") && !strings.HasSuffix(name, "_")
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
