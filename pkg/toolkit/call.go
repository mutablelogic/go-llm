package toolkit

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Call executes a tool or prompt, passing optional resource arguments.
// The key argument may be a string name, an llm.Tool, or an llm.Prompt.
// For tools, the first resource's content is used as the JSON input.
// For prompts, execution is delegated to the handler.
func (tk *toolkit) Call(ctx context.Context, key any, resources ...llm.Resource) (llm.Resource, error) {
	// Resolve key to a tool or prompt.
	var t llm.Tool
	var p llm.Prompt
	switch v := key.(type) {
	case llm.Tool:
		t = v
	case llm.Prompt:
		p = v
	case string:
		found, err := tk.Lookup(ctx, v)
		if err != nil {
			return nil, err
		}
		switch f := found.(type) {
		case llm.Tool:
			t = f
		case llm.Prompt:
			p = f
		}
	default:
		return nil, llm.ErrBadParameter.Withf("key must be a string, llm.Tool, or llm.Prompt, got %T", key)
	}

	// Perform the call
	switch {
	case t != nil:
		return callTool(ctx, t, resources...)
	case p != nil:
		if tk.handler == nil {
			return nil, llm.ErrNotImplemented.With("no handler set for prompt execution")
		}
		return tk.handler.Call(ctx, p, resources...)
	default:
		return nil, llm.ErrNotFound.Withf("%v", key)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// callTool validates and executes a single tool. Only one resource is supported, whose content must be JSON and is passed as input to the tool. The output must be an llm.Resource or nil.
func callTool(ctx context.Context, t llm.Tool, resources ...llm.Resource) (llm.Resource, error) {
	var input json.RawMessage

	// Check for too many resources. Only one is supported as input to the tool.
	if len(resources) > 1 {
		return nil, llm.ErrBadParameter.Withf("too many resources: zero or one is supported as input to a tool, got %d", len(resources))
	} else if len(resources) == 1 && resources[0] == nil {
		return nil, llm.ErrBadParameter.With("resource cannot be nil")
	} else if len(resources) == 1 && resources[0].Type() != types.ContentTypeJSON {
		return nil, llm.ErrBadParameter.Withf("invalid resource type: expected %q, got %q", types.ContentTypeJSON, resources[0].Type())
	} else if len(resources) == 1 {
		data, err := resources[0].Read(ctx)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("reading input resource: %v", err)
		}
		input = json.RawMessage(data)
		s, err := t.InputSchema()
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("schema generation failed: %v", err)
		}
		if s != nil {
			// TODO: Validate input against schema
			var mapInput map[string]any
			if err := json.Unmarshal(input, &mapInput); err != nil {
				return nil, llm.ErrBadParameter.Withf("failed to unmarshal JSON input: %v", err)
			}
			resolved, err := s.Resolve(nil)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("schema resolution failed: %v", err)
			}
			if err := resolved.Validate(mapInput); err != nil {
				return nil, llm.ErrBadParameter.Withf("input validation failed: %v", err)
			}
		}
	}

	// TODO: Set up the session for the call - this one is "builtin" session
	// If this tool is a wrapper for a connector tool, it may set it's own session

	// TODO: Start otel span

	// Execute the tool
	result, err := t.Run(ctx, input)
	if err != nil {
		return nil, err
	}

	// If the result is nil, check the output schema.
	if result == nil {
		outputSchema, err := t.OutputSchema()
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("output schema generation failed: %v", err)
		}
		if outputSchema != nil {
			return nil, llm.ErrBadParameter.With("tool returned nil but an output schema is defined")
		}
		return nil, nil
	}

	// Output must be an llm.Resource.
	resource, ok := result.(llm.Resource)
	if !ok {
		return nil, llm.ErrBadParameter.Withf("tool output must be nil or llm.Resource, got %T", result)
	}

	// If JSON output and an output schema exists, validate the content.
	if resource.Type() == types.ContentTypeJSON {
		outputSchema, err := t.OutputSchema()
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("output schema generation failed: %v", err)
		}
		if outputSchema != nil {
			data, err := resource.Read(ctx)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("failed to read resource for validation: %v", err)
			}
			var mapOutput map[string]any
			if err := json.Unmarshal(data, &mapOutput); err != nil {
				return nil, llm.ErrBadParameter.Withf("failed to unmarshal JSON output: %v", err)
			}
			resolved, err := outputSchema.Resolve(nil)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("output schema resolution failed: %v", err)
			}
			if err := resolved.Validate(mapOutput); err != nil {
				return nil, llm.ErrBadParameter.Withf("output validation failed: %v", err)
			}
		}
	}

	// Return success
	return resource, nil
}
