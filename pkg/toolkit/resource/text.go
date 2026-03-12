package resource

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// textResource is a plain-text resource whose content is held in memory.
type textResource struct {
	name    string
	content string
}

var _ llm.Resource = (*textResource)(nil)

///////////////////////////////////////////////////////////////////////////////
// CONSTRUCTOR

// Text returns a new in-memory text resource with the given name and content.
// The URI scheme is "text:" and the MIME type is "text/plain".
// Name must be a non-empty identifier.
func Text(name, content string) (llm.Resource, error) {
	if !types.IsIdentifier(name) {
		return nil, llm.ErrBadParameter.Withf("name: must be a non-empty identifier, got %q", name)
	}
	return &textResource{name: name, content: content}, nil
}

///////////////////////////////////////////////////////////////////////////////
// llm.Resource INTERFACE

func (t *textResource) URI() string         { return "text:" + t.name }
func (t *textResource) Name() string        { return t.name }
func (t *textResource) Description() string { return "" }
func (t *textResource) Type() string        { return types.ContentTypeTextPlain }

func (t *textResource) Read(_ context.Context) ([]byte, error) {
	return []byte(t.content), nil
}

func (t *textResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		URI  string `json:"uri"`
		Name string `json:"name"`
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}{t.URI(), t.name, t.Type(), t.content})
}

func (t *textResource) String() string { return types.Stringify(t) }
