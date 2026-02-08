package tool

import (
	"context"
	"encoding/json"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-llm/pkg/types"
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
	Schema() (*jsonschema.Schema, error)

	// Run the tool with the given input as JSON (may be nil)
	Run(ctx context.Context, input json.RawMessage) (any, error)
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
		schema, err := tool.Schema()
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

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (tk *Toolkit) String() string {
	return types.Stringify(tk.Tools())
}
