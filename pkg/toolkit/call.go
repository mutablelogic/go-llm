package toolkit

import (
	"context"
	"encoding/json"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	types "github.com/mutablelogic/go-server/pkg/types"
	gootel "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	propagation "go.opentelemetry.io/otel/propagation"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Call executes a tool or prompt, passing optional resource arguments.
// The key argument may be a string name, an llm.Tool, or an llm.Prompt.
// For tools, the first resource's content is used as the JSON input.
// For prompts, execution is delegated to the delegate.
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
		return tk.callTool(ctx, t, resources...)
	case p != nil:
		if tk.delegate == nil {
			return nil, llm.ErrNotImplemented.With("no delegate set for prompt execution")
		}
		return tk.delegate.Call(ctx, p, resources...)
	default:
		return nil, llm.ErrNotFound.Withf("%v", key)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// callTool validates and executes a single tool. Only one resource is supported, whose content must be JSON and is passed as input to the tool. The output must be an llm.Resource or nil.
func (tk *toolkit) callTool(ctx context.Context, t llm.Tool, resources ...llm.Resource) (_ llm.Resource, spanErr error) {
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
			var instance any
			if err := json.Unmarshal(input, &instance); err != nil {
				return nil, llm.ErrBadParameter.Withf("failed to unmarshal JSON input: %v", err)
			}
			resolved, err := s.Resolve(nil)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("schema resolution failed: %v", err)
			}
			if err := resolved.Validate(instance); err != nil {
				return nil, llm.ErrBadParameter.Withf("input validation failed: %v", err)
			}
		}
	}

	// Start otel span
	otelCtx, spanEnd := otel.StartSpan(tk.tracer, ctx, t.Name(), attribute.String("input", string(input)))
	defer func() { spanEnd(spanErr) }()

	// Set traceparent in the meta for potential downstream propagation
	meta := metaFromContext(ctx)
	carrier := propagation.MapCarrier{}
	gootel.GetTextMapPropagator().Inject(otelCtx, carrier)
	for k, v := range carrier {
		meta = append(meta, schema.MetaValue{Key: k, Value: v})
	}

	// Set a session and then execute the tool
	result, err := t.Run(withSessionContext(otelCtx, tk.newSession(t.Name(), meta...)), input)
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

	// Wrap common non-Resource return types into an appropriate llm.Resource.
	// Tools like OutputTool return json.RawMessage directly; string and []byte
	// are also accepted as convenience types.
	// Unwrap any namespace wrapper to get the bare tool name for the resource.
	type unwrapper interface{ Unwrap() llm.Tool }
	baseTool := t
	for {
		u, ok := baseTool.(unwrapper)
		if !ok {
			break
		}
		baseTool = u.Unwrap()
	}
	var wrapped llm.Resource
	switch v := result.(type) {
	case llm.Resource:
		wrapped = v
	case json.RawMessage:
		r, err := resource.JSON(baseTool.Name(), v)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("wrapping json.RawMessage output: %v", err)
		}
		wrapped = r
	case []byte:
		r, err := resource.Data(baseTool.Name(), v)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("wrapping []byte output: %v", err)
		}
		wrapped = r
	case string:
		r, err := resource.Text(baseTool.Name(), v)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("wrapping string output: %v", err)
		}
		wrapped = r
	default:
		return nil, llm.ErrBadParameter.Withf("tool output must be nil, llm.Resource, json.RawMessage, []byte, or string, got %T", result)
	}

	// If there isn't an output schema, return the wrapped resource as-is.
	outputSchema, err := t.OutputSchema()
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("output schema generation failed: %v", err)
	}
	if outputSchema == nil {
		return wrapped, nil
	}

	// If not JSON output return an error since we won't be able to validate against the schema.
	if wrapped.Type() != types.ContentTypeJSON {
		return nil, llm.ErrBadParameter.Withf("output validation failed: output schema is defined but tool did not return JSON content (got %q)", wrapped.Type())
	}

	// Validate the wrapped resource content against the output schema.
	data, err := wrapped.Read(ctx)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("failed to read resource for validation: %v", err)
	}
	var instance any
	if err := json.Unmarshal(data, &instance); err != nil {
		return nil, llm.ErrBadParameter.Withf("failed to unmarshal JSON output: %v", err)
	}
	resolved, err := outputSchema.Resolve(nil)
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("output schema resolution failed: %v", err)
	}
	if err := resolved.Validate(instance); err != nil {
		return nil, llm.ErrBadParameter.Withf("output validation failed: %v", err)
	}

	// Return success
	return wrapped, nil
}
