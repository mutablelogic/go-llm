package tool

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
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
	// Set defaults
	tk := &Toolkit{
		builtins: make(map[string]llm.Tool),
		conns:    make(map[string]*connEntry),
		onLog:    func(string, slog.Level, string, ...any) {},
		onState:  func(string, schema.ConnectorState) {},
		onTools:  func(string, []llm.Tool) {},
	}

	// Apply options
	for _, o := range opts {
		if err := o(tk); err != nil {
			return nil, err
		}
	}

	// Return the toolkit
	return tk, nil
}

// Close cancels all active connector goroutines, waits for them to finish,
// and releases resources.
func (tk *Toolkit) Close() error {
	// Disconnect all connectors
	tk.mu.Lock()
	for _, e := range tk.conns {
		e.cancel()
	}
	clear(tk.conns)
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
func (tk *Toolkit) ListTools(req schema.ToolListRequest) []llm.Tool {
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

	// Snapshot builtins
	var result []llm.Tool
	if req.Namespace == "" || req.Namespace == schema.BuiltinNamespace {
		for _, t := range tk.builtins {
			if !isReservedToolName(t.Name()) && matchName(t.Name()) {
				result = append(result, t)
			}
		}
	}

	// Snapshot connector refs
	type connRef struct {
		url  string
		conn llm.Connector
	}
	var refs []connRef
	if req.Namespace != schema.BuiltinNamespace {
		for url, entry := range tk.conns {
			if req.Namespace != "" && req.Namespace != url {
				continue
			}
			refs = append(refs, connRef{url: url, conn: entry.connector})
		}
	}

	tk.mu.RUnlock()

	// Query connectors outside the lock
	for _, ref := range refs {
		tools, err := ref.conn.ListTools(context.Background())
		if err != nil {
			continue
		}
		for _, t := range tools {
			if !isReservedToolName(t.Name()) && matchName(t.Name()) {
				result = append(result, t)
			}
		}
	}

	return result
}

// AddBuiltin adds one or more locally-implemented tools to the toolkit.
// Returns an error if any tool has an invalid or duplicate name,
// or if the name is reserved (e.g. "submit_output").
func (tk *Toolkit) AddBuiltin(tools ...llm.Tool) error {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	for _, t := range tools {
		name := t.Name()
		if !types.IsIdentifier(name) {
			return schema.ErrBadParameter.Withf("invalid tool name: %q", name)
		}
		// Reject reserved names unless the tool is the internal OutputTool
		if isReservedToolName(name) {
			if _, ok := t.(*OutputTool); !ok {
				return schema.ErrBadParameter.Withf("reserved tool name: %q", name)
			}
		}
		if _, exists := tk.builtins[name]; exists {
			return schema.ErrBadParameter.Withf("duplicate tool name: %q", name)
		}
		tk.builtins[name] = t
	}
	return nil
}

// isReservedToolName returns true if the name is reserved for internal use.
func isReservedToolName(name string) bool {
	return name == OutputToolName
}

// Lookup returns a tool by name, searching builtins first then connector tools.
// Returns nil if not found.
func (tk *Toolkit) Lookup(name string) llm.Tool {
	tk.mu.RLock()
	if t, ok := tk.builtins[name]; ok {
		tk.mu.RUnlock()
		return t
	}
	// Collect connectors to query without holding lock
	conns := make([]llm.Connector, 0, len(tk.conns))
	for _, entry := range tk.conns {
		conns = append(conns, entry.connector)
	}
	tk.mu.RUnlock()

	for _, conn := range conns {
		tools, err := conn.ListTools(context.Background())
		if err != nil {
			continue
		}
		for _, t := range tools {
			if t.Name() == name {
				return t
			}
		}
	}
	return nil
}

// Run executes a tool by name with the given JSON input.
// Returns an error if the tool is not found, the input does not match the schema,
// or the tool execution fails.
func (tk *Toolkit) Run(ctx context.Context, name string, input json.RawMessage) (any, error) {
	// Lookup the tool
	tool := tk.Lookup(name)
	if tool == nil {
		return nil, schema.ErrNotFound.Withf("tool not found: %q", name)
	}

	// Validate input against schema if provided
	if len(input) > 0 {
		inputSchema := tool.InputSchema()
		if inputSchema != nil {
			if err := inputSchema.Validate(input); err != nil {
				return nil, schema.ErrBadParameter.Withf("input validation failed: %v", err)
			}
		}
	}

	// Run the tool
	result, err := tool.Run(ctx, input)
	if err != nil {
		return nil, err
	}

	// Validate output: must be nil or llm.Resource
	if result == nil {
		return nil, nil
	}
	resource, ok := result.(llm.Resource)
	if !ok {
		return nil, schema.ErrBadParameter.Withf("tool output must be nil or llm.Resource, got %T", result)
	}

	// If application/json and output schema exists, validate content
	if resource.Type() == types.ContentTypeJSON {
		outputSchema := tool.OutputSchema()
		if outputSchema != nil {
			data, err := resource.Read(ctx)
			if err != nil {
				return nil, schema.ErrBadParameter.Withf("failed to read resource for validation: %v", err)
			}
			if err := outputSchema.Validate(data); err != nil {
				return nil, schema.ErrBadParameter.Withf("output validation failed: %v", err)
			}
		}
	}

	return resource, nil
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
	return types.Stringify(tk.ListTools(schema.ToolListRequest{}))
}
