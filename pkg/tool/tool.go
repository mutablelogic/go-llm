package tool

import (
	"context"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Tool is an interface for a tool with a name, description and JSON schema
type Tool interface {
	// Return the name of the tool
	Name() string

	// Return the description of the tool
	Description() string

	// Return the JSON schema for the tool input
	Schema() *jsonschema.Schema

	// Run the tool with the given input
	Run(ctx context.Context, input any) (any, error)
}

// Toolkit is a collection of tools with unique names
type Toolkit struct {
	tools map[string]Tool
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewToolkit creates a new toolkit with the given tools.
// Returns an error if any tool has an invalid or duplicate name.
func NewToolkit(tools ...Tool) (*Toolkit, error) {
	tk := &Toolkit{
		tools: make(map[string]Tool),
	}
	if err := tk.Register(tools...); err != nil {
		return nil, err
	}
	return tk, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Tools returns all tools in the toolkit
func (tk *Toolkit) Tools() []Tool {
	result := make([]Tool, 0, len(tk.tools))
	for _, t := range tk.tools {
		result = append(result, t)
	}
	return result
}

// Register adds one or more tools to the toolkit.
// Returns an error if any tool has an invalid or duplicate name.
func (tk *Toolkit) Register(tools ...Tool) error {
	for _, t := range tools {
		name := t.Name()
		if !types.IsIdentifier(name) {
			return llm.ErrBadParameter.Withf("invalid tool name: %q", name)
		}
		if _, exists := tk.tools[name]; exists {
			return llm.ErrBadParameter.Withf("duplicate tool name: %q", name)
		}
		tk.tools[name] = t
	}
	return nil
}

// Lookup returns a tool by name, or nil if not found
func (tk *Toolkit) Lookup(name string) Tool {
	return tk.tools[name]
}

// Run executes a tool by name with the given input.
// Returns an error if the tool is not found, the input does not match the schema,
// or the tool execution fails.
func (tk *Toolkit) Run(ctx context.Context, name string, input any) (any, error) {
	// Lookup the tool
	tool := tk.Lookup(name)
	if tool == nil {
		return nil, llm.ErrNotFound.Withf("tool not found: %q", name)
	}

	// Validate input against schema
	if schema := tool.Schema(); schema != nil {
		resolved, err := schema.Resolve(nil)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("schema resolution failed: %v", err)
		}
		if err := resolved.Validate(input); err != nil {
			return nil, llm.ErrBadParameter.Withf("input validation failed: %v", err)
		}
	}

	// Run the tool
	return tool.Run(ctx, input)
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (tk *Toolkit) String() string {
	return schema.Stringify(tk.Tools())
}
