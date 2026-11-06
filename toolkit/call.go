package toolkit

import (
	"bytes"
	"context"
	"encoding/json"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	attribute "go.opentelemetry.io/otel/attribute"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CallTool runs the tool with the name, which is its canonical name, such as
// "weatherapi:current", or its name without a namespace, such as "current",
// with the input, which may be empty, and returns its result as JSON, or nil
// when the tool has no result. Returns ErrNotFound if there is no tool with
// the name, ErrConflict if the name has no namespace, and more than one
// namespace has a tool with the name, or ErrBadParameter if the input isn't
// valid JSON, or doesn't match the tool's input schema. The tool is run
// without holding the toolkit's lock, so tools can be added and removed while
// it runs.
func (self *Toolkit) CallTool(ctx context.Context, name string, input json.RawMessage) (_ json.RawMessage, err error) {
	ctx, endSpan := otel.StartSpan(self.tracer, ctx, "llm.toolkit.CallTool",
		attribute.String("arg.name", name),
	)
	defer func() { endSpan(err) }()

	// Get the tool, and call it
	entry, err := self.lookup(name)
	if err != nil {
		return nil, err
	}
	return entry.call(ctx, input)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// call runs the tool with the input, which may be empty, and returns its
// result as JSON, or nil when the tool has no result. Input which doesn't
// match the tool's input schema has its fields which are null left out, as
// models send null for the fields they don't set, which a schema rejects,
// as they're optional, rather than nullable, so input which matches the
// schema, with its nulls, such as a field which is nullable, is as it is,
// as is the input of a tool without a schema. Returns ErrBadParameter if the
// input isn't valid JSON, or doesn't match the tool's input schema.
func (e entry) call(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	name := schema.CanonicalToolName(e.namespace, e.tool.Name())

	// The input, if any, must be valid JSON
	if len(input) > 0 && !json.Valid(input) {
		return nil, llm.ErrBadParameter.Withf("tool %q: input isn't valid JSON", name)
	}

	// The input must match the tool's input schema, if it has one, as it is,
	// or without its fields which are null
	if e.input != nil {
		if err := e.validate(input); err != nil {
			stripped := withoutNulls(input)
			if bytes.Equal(stripped, input) || e.validate(stripped) != nil {
				return nil, llm.ErrBadParameter.Withf("tool %q: %v", name, err)
			}
			input = stripped
		}
	}

	// Run the tool, and return its result as JSON
	result, err := e.tool.Run(ctx, input)
	if err != nil {
		return nil, err
	}
	return toolResult(ctx, name, result)
}

// validate returns an error when the input doesn't match the tool's input
// schema. Empty input is validated as an empty object.
func (e entry) validate(input json.RawMessage) error {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	return e.input.Validate(input)
}

// withoutNulls returns the input without the fields of an object which are
// null, so they're left out, as a model sends null for the fields of a tool's
// input which it doesn't set, which an input schema rejects, as the fields
// are optional, rather than nullable. Input which isn't an object, or has no
// fields which are null, is returned as it is.
func withoutNulls(input json.RawMessage) json.RawMessage {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil || fields == nil {
		return input
	}
	removed := false
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			delete(fields, key)
			removed = true
		}
	}
	if !removed {
		return input
	} else if data, err := json.Marshal(fields); err == nil {
		return data
	}
	return input
}

// toolResult returns the result of the tool as JSON. JSON must be valid, but
// isn't checked against the tool's output schema. Resources, which a tool
// returns on their own, or with a value, as a Result, are kept, with the
// context's store function, and the result has links to them, and the
// value, or, without one, their types and data, and any other result is
// encoded as JSON, so text is a JSON string. A nil or empty result is no
// result, and is returned as nil.
func toolResult(ctx context.Context, name string, result any) (json.RawMessage, error) {
	switch v := result.(type) {
	case nil:
		return nil, nil
	case json.RawMessage:
		if len(v) == 0 {
			return nil, nil
		} else if !json.Valid(v) {
			return nil, llm.ErrInternalServerError.Withf("tool %q: result isn't valid JSON", name)
		}
		return v, nil
	case llm.Resource:
		return resourcesResult(ctx, name, nil, []llm.Resource{v})
	case []llm.Resource:
		return resourcesResult(ctx, name, nil, v)
	case Result:
		return resourcesResult(ctx, name, v.Value, v.Resources)
	case *Result:
		if v == nil {
			return nil, nil
		}
		return resourcesResult(ctx, name, v.Value, v.Resources)
	default:
		return marshalResult(name, v)
	}
}

// resourcesResult returns the result of a tool which returned resources,
// with the value, when there is one. With the context's store function,
// they're kept, and the result has links to them. Without one, a resource on
// its own is its type and data, as it was before resources were kept, and
// otherwise each resource is its type and data, in the result's resources.
func resourcesResult(ctx context.Context, name string, value any, resources []llm.Resource) (json.RawMessage, error) {
	if store := storeFrom(ctx); store != nil {
		links := make([]schema.ResourceLink, 0, len(resources))
		for _, resource := range resources {
			if resource == nil {
				continue
			}
			link, err := store(ctx, resource)
			if err != nil {
				return nil, err
			}
			links = append(links, link)
		}
		return marshalResult(name, schema.ResourcesResult{Result: value, Resources: links})
	}

	// Without one, their types and data
	inline := make([]providerschema.Resource, 0, len(resources))
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		data, err := llm.ReadAll(ctx, resource)
		if err != nil {
			return nil, err
		}
		inline = append(inline, providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: resource.Type(), Data: data}})
	}
	if value == nil && len(inline) == 1 {
		return marshalResult(name, inline[0])
	}
	return marshalResult(name, struct {
		Result    any                       `json:"result,omitempty"`
		Resources []providerschema.Resource `json:"resources"`
	}{value, inline})
}

// marshalResult returns the result of the tool encoded as JSON
func marshalResult(name string, v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, llm.ErrInternalServerError.Withf("tool %q: %v", name, err)
	}
	return data, nil
}
