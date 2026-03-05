package tool

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TOOLKIT

// Toolkit is a collection of tools with unique names
type Toolkit struct {
	mu       sync.RWMutex
	wg       sync.WaitGroup
	builtins map[string]llm.Tool
	conns    map[string]*connEntry

	// Callbacks
	onLog   func(url string, level slog.Level, msg string, args ...any)
	onState func(url string, state schema.ConnectorState)
	onTools func(url string, tools []llm.Tool)
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewToolkit creates a new toolkit with the given options.
func NewToolkit(opts ...ToolkitOpt) (*Toolkit, error) {
	tk := &Toolkit{
		builtins: make(map[string]llm.Tool),
		conns:    make(map[string]*connEntry),
		onLog:    func(string, slog.Level, string, ...any) {},
		onState:  func(string, schema.ConnectorState) {},
		onTools:  func(string, []llm.Tool) {},
	}
	for _, o := range opts {
		if err := o(tk); err != nil {
			return nil, err
		}
	}
	return tk, nil
}

// Close cancels all active connector goroutines, waits for them to finish,
// and releases resources.
func (tk *Toolkit) Close() error {
	tk.mu.Lock()
	for _, e := range tk.conns {
		e.cancel()
	}
	tk.conns = make(map[string]*connEntry)
	tk.mu.Unlock()

	// Wait for all connectors to disconnect
	tk.wg.Wait()

	// Return success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools returns tools matching the given request filters.
// An empty request returns all tools across all namespaces (builtins + connectors).
func (tk *Toolkit) ListTools(req schema.ListToolsRequest) []llm.Tool {
	// Build a name-filter set for O(1) lookup; nil means no filter.
	var nameSet map[string]struct{}
	if len(req.Name) > 0 {
		nameSet = make(map[string]struct{}, len(req.Name))
		for _, n := range req.Name {
			nameSet[n] = struct{}{}
		}
	}
	matchName := func(name string) bool {
		if nameSet == nil {
			return true
		}
		_, ok := nameSet[name]
		return ok
	}

	tk.mu.RLock()
	defer tk.mu.RUnlock()

	var result []llm.Tool

	// Builtins
	if req.Namespace == "" || req.Namespace == schema.BuiltinNamespace {
		for _, t := range tk.builtins {
			if !isReservedToolName(t.Name()) && matchName(t.Name()) {
				result = append(result, t)
			}
		}
	}

	// Connector tools
	if req.Namespace != schema.BuiltinNamespace {
		for url, entry := range tk.conns {
			if req.Namespace != "" && req.Namespace != url {
				continue
			}
			for _, t := range entry.tools {
				if !isReservedToolName(t.Name()) && matchName(t.Name()) {
					result = append(result, t)
				}
			}
		}
	}

	return result
}

// AddBuiltin adds one or more locally-implemented tools to the toolkit.
// Returns an error if any tool has an invalid or duplicate name,
// or if the name is reserved (e.g. "submit_output").
func (tk *Toolkit) AddBuiltin(tools ...llm.Tool) error {
	for _, t := range tools {
		name := t.Name()
		if !types.IsIdentifier(name) {
			return llm.ErrBadParameter.Withf("invalid tool name: %q", name)
		}
		// Reject reserved names unless the tool is the internal OutputTool
		if isReservedToolName(name) {
			if _, ok := t.(*OutputTool); !ok {
				return llm.ErrBadParameter.Withf("reserved tool name: %q", name)
			}
		}
		if _, exists := tk.builtins[name]; exists {
			return llm.ErrBadParameter.Withf("duplicate tool name: %q", name)
		}
		tk.builtins[name] = t
	}
	return nil
}

// isReservedToolName returns true if the name is reserved for internal use.
func isReservedToolName(name string) bool {
	return name == OutputToolName
}

// Lookup returns a builtin tool by name, or nil if not found
func (tk *Toolkit) Lookup(name string) llm.Tool {
	return tk.builtins[name]
}

// Run executes a tool by name with the given input.
// The input should be json.RawMessage or nil.
// Returns an error if the tool is not found, the input does not match the schema,
// or the tool execution fails.
func (tk *Toolkit) Run(ctx context.Context, name string, input any) (any, error) {
	// Lookup the tool
	tool := tk.Lookup(name)
	if tool == nil {
		return nil, llm.ErrNotFound.Withf("tool not found: %q", name)
	}

	// Convert input to json.RawMessage
	var rawInput json.RawMessage
	if input != nil {
		switch v := input.(type) {
		case json.RawMessage:
			rawInput = v
		case []byte:
			rawInput = json.RawMessage(v)
		default:
			// If not JSON, marshal it
			data, err := json.Marshal(input)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("failed to marshal input: %v", err)
			}
			rawInput = json.RawMessage(data)
		}
	}

	// Validate input against schema if provided
	if len(rawInput) > 0 {
		schema, err := tool.InputSchema()
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("schema generation failed: %v", err)
		}

		if schema != nil {
			// Unmarshal into a map for validation
			var mapInput map[string]any
			if err := json.Unmarshal(rawInput, &mapInput); err != nil {
				return nil, llm.ErrBadParameter.Withf("failed to unmarshal JSON input: %v", err)
			}

			// Validate against schema
			resolved, err := schema.Resolve(nil)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("schema resolution failed: %v", err)
			}
			if err := resolved.Validate(mapInput); err != nil {
				return nil, llm.ErrBadParameter.Withf("input validation failed: %v", err)
			}
		}
	}

	// Run the tool with raw JSON
	return tool.Run(ctx, rawInput)
}

// Feedback returns a human-readable description of a tool call, including
// the tool name and its description when available.
func (tk *Toolkit) Feedback(call schema.ToolCall) string {
	if t := tk.Lookup(call.Name); t != nil && t.Description() != "" {
		return call.Name + ": " + t.Description()
	}
	return call.Name
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (tk *Toolkit) String() string {
	return types.Stringify(tk.ListTools(schema.ListToolsRequest{}))
}
