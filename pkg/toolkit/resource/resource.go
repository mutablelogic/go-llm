package resource

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// namespacedResource wraps an llm.Resource and prepends a namespace to its name.
type namespacedResource struct {
	llm.Resource
	name string
}

// describedResource wraps an llm.Resource and overrides its description.
type describedResource struct {
	llm.Resource
	description string
}

var _ llm.Resource = (*namespacedResource)(nil)
var _ llm.Resource = (*describedResource)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithNamespace returns r with its name replaced by "namespace.r.Name()".
func WithNamespace(namespace string, r llm.Resource) llm.Resource {
	return &namespacedResource{
		Resource: r,
		name:     namespace + "." + r.Name(),
	}
}

// WithDescription returns r with its description replaced by the given value.
func WithDescription(description string, r llm.Resource) llm.Resource {
	return &describedResource{Resource: r, description: description}
}

func (n *namespacedResource) Name() string       { return n.name }
func (d *describedResource) Description() string { return d.description }

///////////////////////////////////////////////////////////////////////////////
// json.Marshaler

type resourceJSON struct {
	URI         string          `json:"uri"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data,omitempty"`
}

func marshalResource(r llm.Resource) ([]byte, error) {
	var data json.RawMessage
	// Unwrap wrappers to reach the concrete resource that owns the data.
	if m, ok := unwrapResource(r).(json.Marshaler); ok {
		inner, err := m.MarshalJSON()
		if err != nil {
			return nil, err
		}
		var v resourceJSON
		if err := json.Unmarshal(inner, &v); err != nil {
			return nil, err
		} else {
			data = v.Data
		}
	}
	return json.Marshal(resourceJSON{
		URI:         r.URI(),
		Name:        r.Name(),
		Description: r.Description(),
		Type:        r.Type(),
		Data:        data,
	})
}

// unwrapResource peels off wrapper types to find the innermost concrete resource.
func unwrapResource(r llm.Resource) llm.Resource {
	for {
		switch w := r.(type) {
		case *namespacedResource:
			r = w.Resource
		case *describedResource:
			r = w.Resource
		default:
			return r
		}
	}
}

func (n *namespacedResource) MarshalJSON() ([]byte, error) { return marshalResource(n) }
func (d *describedResource) MarshalJSON() ([]byte, error)  { return marshalResource(d) }

///////////////////////////////////////////////////////////////////////////////
// json.Unmarshaler

// Unmarshal decodes a JSON-encoded resource (as produced by MarshalJSON) back
// into an llm.Resource. The MIME type field determines the concrete type:
//   - "text/plain"       → text resource
//   - "application/json" → JSON resource
//   - anything else      → binary data resource
//
// If the JSON contains a non-empty description the result is wrapped with
// WithDescription so that Description() returns the stored value.
func Unmarshal(data []byte) (llm.Resource, error) {
	var v resourceJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	var r llm.Resource
	switch v.Type {
	case "text/plain":
		var content string
		if len(v.Data) > 0 {
			if err := json.Unmarshal(v.Data, &content); err != nil {
				return nil, llm.ErrBadParameter.Withf("text data: %v", err)
			}
		}
		r = &textResource{name: v.Name, content: content}
	case "application/json":
		r = &jsonResource{name: v.Name, content: json.RawMessage(v.Data)}
	default:
		var content []byte
		if len(v.Data) > 0 {
			if err := json.Unmarshal(v.Data, &content); err != nil {
				return nil, llm.ErrBadParameter.Withf("data: %v", err)
			}
		}
		r = &dataResource{name: v.Name, mimetype: v.Type, content: content}
	}

	if v.Description != "" {
		r = WithDescription(v.Description, r)
	}
	return r, nil
}
