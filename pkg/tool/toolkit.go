package tool

import (
	"context"
	"errors"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ToolKit represents a toolkit of tools
type ToolKit struct {
	functions map[string]tool
}

var _ llm.ToolKit = (*ToolKit)(nil)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new empty toolkit for an agent
func NewToolKit() *ToolKit {
	return &ToolKit{
		functions: make(map[string]tool),
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return all registered tools for a specific agent. Set agent to nil
// for standard tool format.
func (kit *ToolKit) Tools(agentName string) []llm.Tool {
	result := make([]llm.Tool, 0, len(kit.functions))
	for _, t := range kit.functions {
		switch {
		case agentName == "mcp":
			t.Parameters = nil
			t.InputSchema = nil
			result = append(result, t)
		case agentName == "anthropic":
			t.Parameters = nil
			t.InputSchema2 = nil
			result = append(result, t)
		default:
			t.InputSchema = nil
			t.InputSchema2 = nil
			result = append(result, ToolFunction{
				Type: "function",
				Tool: t,
			})
		}
	}
	return result
}

// Register a tool in the toolkit
func (kit *ToolKit) Register(v llm.Tool) error {
	if v == nil {
		return llm.ErrBadParameter.With("tool cannot be nil")
	}

	name := v.Name()
	if _, exists := kit.functions[name]; exists {
		return llm.ErrConflict.Withf("tool %q already exists", name)
	}

	// Set the tool
	t := tool{
		Tool: v,
		ToolMeta: ToolMeta{
			Name:        name,
			Description: v.Description(),
		},
	}

	// Determine parameters
	toolparams, err := paramsFor(nil, v)
	if err != nil {
		return err
	}

	// Add parameters
	parameters := ToolParameters{
		Type:       "object",
		Required:   make([]string, 0, len(toolparams)),
		Properties: make(map[string]ToolParameter, len(toolparams)),
	}

	// Set parameters
	for _, param := range toolparams {
		if _, exists := parameters.Properties[param.Name]; exists {
			return llm.ErrConflict.Withf("parameter %q already exists", param.Name)
		} else {
			parameters.Properties[param.Name] = param
		}
		if param.required {
			parameters.Required = append(parameters.Required, param.Name)
		}
	}

	// The three cases of parameters (doh!) are "input_schema", "inputSchema" and "parameters"
	t.Parameters = &parameters
	t.InputSchema = &parameters
	t.InputSchema2 = &parameters

	// Add to toolkit
	kit.functions[name] = t

	// Return success
	return nil
}

// Run calls a tool in the toolkit
func (kit *ToolKit) Run(ctx context.Context, calls ...llm.ToolCall) ([]llm.ToolResult, error) {
	var wg sync.WaitGroup
	var errs error
	var toolresult []llm.ToolResult

	// TODO: Lock each tool so it can only be run in series (although different
	// tools can be run in parallel)
	for _, call := range calls {
		wg.Add(1)
		go func(call llm.ToolCall) {
			defer wg.Done()

			// Get the tool and run it
			name := call.Name()
			if _, exists := kit.functions[name]; !exists {
				errs = errors.Join(errs, llm.ErrNotFound.Withf("tool %q not found", name))
			} else if err := call.Decode(kit.functions[name].Tool); err != nil {
				errs = errors.Join(errs, err)
			} else if out, err := kit.functions[name].Tool.Run(ctx); err != nil {
				errs = errors.Join(errs, err)
			} else {
				toolresult = append(toolresult, NewResult(call, out))
			}
		}(call)
	}

	// Wait for all calls to complete
	wg.Wait()

	// Return any errors
	return toolresult, errs
}
