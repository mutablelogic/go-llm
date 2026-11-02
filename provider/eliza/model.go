package eliza

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model is one of eliza's languages
type Model struct {
	provider *Client
	lang     *Language
}

var _ llm.Model = (*Model)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ModelName is an alias of eliza's English model, which is its default
const ModelName = "eliza"

// defaultModel is the name of eliza's English model
const defaultModel = "eliza-1966-en"

// created is when the real ELIZA chatbot was first described, by Joseph
// Weizenbaum at MIT.
var created = time.Date(1966, 1, 1, 0, 0, 0, 0, time.UTC)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *Model) Name() string {
	return m.lang.Model
}

func (m *Model) Provider() llm.Provider {
	return m.provider
}

func (m *Model) Description() string {
	return m.lang.Description
}

func (m *Model) Created() time.Time {
	return created
}

// Aliases returns eliza for the English model, which is its default
func (m *Model) Aliases() []string {
	if m.lang.Model == defaultModel {
		return []string{ModelName}
	}
	return nil
}

// Meta returns who created ELIZA, and the model's language
func (m *Model) Meta() map[string]json.RawMessage {
	meta := map[string]json.RawMessage{
		"author":      json.RawMessage(`"Joseph Weizenbaum"`),
		"institution": json.RawMessage(`"MIT"`),
		"year":        json.RawMessage(`1966`),
	}
	if language, err := json.Marshal(m.lang.LanguageCode); err == nil {
		meta["language"] = language
	}
	return meta
}

// InputTokenLimit is not meaningful for a mock model.
func (m *Model) InputTokenLimit() *uint {
	return nil
}

// OutputTokenLimit is not meaningful for a mock model.
func (m *Model) OutputTokenLimit() *uint {
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER METHODS

// ListModels returns a model for each of eliza's languages, in order of
// their names
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	names := make([]string, 0, len(c.languages))
	for name := range c.languages {
		names = append(names, name)
	}
	slices.Sort(names)
	models := make([]llm.Model, 0, len(names))
	for _, name := range names {
		models = append(models, &Model{provider: c, lang: c.languages[name]})
	}
	return models, nil
}

// GetModel returns the model with the name, or the English model for
// eliza, or a not-found error
func (c *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	if name == ModelName {
		name = defaultModel
	}
	if lang, ok := c.languages[name]; ok {
		return &Model{provider: c, lang: lang}, nil
	}
	return nil, llm.ErrNotFound.Withf("model %q not found", name)
}
