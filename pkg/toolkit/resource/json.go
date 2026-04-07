package resource

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// jsonResource is an in-memory JSON resource.
type jsonResource struct {
	name    string
	content json.RawMessage
}

var _ llm.Resource = (*jsonResource)(nil)

///////////////////////////////////////////////////////////////////////////////
// CONSTRUCTOR

// JSON returns a new in-memory JSON resource with the given name.
// If v is already a json.RawMessage it is used directly; otherwise v is
// marshalled to JSON. Name must be a non-empty identifier.
func JSON(name string, v any) (llm.Resource, error) {
	if !types.IsIdentifier(name) {
		return nil, schema.ErrBadParameter.Withf("name: must be a non-empty identifier, got %q", name)
	}
	var data json.RawMessage
	switch val := v.(type) {
	case json.RawMessage:
		data = val
	default:
		var err error
		if data, err = json.Marshal(v); err != nil {
			return nil, schema.ErrBadParameter.Withf("json: %v", err)
		}
	}
	return &jsonResource{name: name, content: data}, nil
}

///////////////////////////////////////////////////////////////////////////////
// llm.Resource INTERFACE

func (j *jsonResource) URI() string         { return "json:" + j.name }
func (j *jsonResource) Name() string        { return j.name }
func (j *jsonResource) Description() string { return "" }
func (j *jsonResource) Type() string        { return types.ContentTypeJSON }

func (j *jsonResource) Read(_ context.Context) ([]byte, error) {
	return j.content, nil
}

func (j *jsonResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		URI  string          `json:"uri"`
		Name string          `json:"name"`
		Type string          `json:"type"`
		Text json.RawMessage `json:"text,omitempty"`
	}{j.URI(), j.name, j.Type(), j.content})
}

func (j *jsonResource) String() string { return types.Stringify(j) }
