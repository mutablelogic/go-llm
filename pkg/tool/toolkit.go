package tool

import (
	// Packages
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ToolKit represents a toolkit of tools
type ToolKit struct {
	functions map[string]tool
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new empty toolkit
func NewToolKit() *ToolKit {
	return &ToolKit{
		functions: make(map[string]tool),
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return all registered tools
func (kit *ToolKit) Tools() []llm.Tool {
	result := make([]llm.Tool, 0, len(kit.functions))
	for _, t := range kit.functions {
		result = append(result, t)
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
		ToolMeta: ToolMeta{
			Name:        name,
			Description: v.Description(),
		},
		proto: reflect.TypeOf(v),
	}

	// Add parameters
	t.Parameters.Type = "object"
	toolparams, err := paramsFor(v)
	if err != nil {
		return err
	}

	// Set parameters
	t.Parameters.Required = make([]string, 0, len(toolparams))
	t.Parameters.Properties = make(map[string]ToolParameter, len(toolparams))
	for _, param := range toolparams {
		if _, exists := t.Parameters.Properties[param.Name]; exists {
			return llm.ErrConflict.Withf("parameter %q already exists", param.Name)
		} else {
			t.Parameters.Properties[param.Name] = param
		}
		if param.required {
			t.Parameters.Required = append(t.Parameters.Required, param.Name)
		}
	}

	// Add to toolkit
	kit.functions[name] = t

	// Return success
	return nil
}

// Run calls a tool in the toolkit
func (kit *ToolKit) Run(ctx context.Context, calls []llm.ToolCall) error {
	var wg sync.WaitGroup
	var result error

	for _, call := range calls {
		wg.Add(1)
		go func(call llm.ToolCall) {
			defer wg.Done()

			// Get the tool
			name := call.Name()
			t, exists := kit.functions[name]
			if !exists {
				result = errors.Join(result, llm.ErrNotFound.Withf("tool %q not found", name))
			}

			// Make a new object to decode into
			v := reflect.New(t.proto).Interface()

			// Decode the input and run the tool
			if err := call.Decode(&v); err != nil {
				result = errors.Join(result, err)
			} else if out, err := t.Run(ctx, v); err != nil {
				result = errors.Join(result, err)
			} else {
				fmt.Println("result of calling", call, "is", out)
			}
		}(call)
	}

	// Wait for all calls to complete
	wg.Wait()

	// Return any errors
	return result
}
