package anthropic

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model is one of Anthropic's models, satisfying the llm.Model interface
type Model struct {
	provider *Client
	data     apiModel
	caps     modelCapabilities
}

var _ llm.Model = (*Model)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// modelsPageSize is how many models are listed at once, which is the most
// the API lists
const modelsPageSize = 1000

// yes and no are capabilities which a model has, or hasn't
var yes, no = types.Ptr(true), types.Ptr(false)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newModel returns the model, with its capabilities, of those the client
// uses, which are none, when they can't be read
func newModel(provider *Client, data apiModel) *Model {
	model := &Model{provider: provider, data: data}
	if len(data.Capabilities) > 0 {
		_ = json.Unmarshal(data.Capabilities, &model.caps)
	}
	return model
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MODEL

// Name returns the model's identifier, such as claude-opus-5-5
func (m *Model) Name() string {
	return m.data.ID
}

// Provider returns the model's provider
func (m *Model) Provider() llm.Provider {
	return m.provider
}

// Description returns the model's name for people, such as Claude Opus 5.5
func (m *Model) Description() string {
	return m.data.DisplayName
}

// Created returns when the model was created, or the zero time, when it
// isn't known
func (m *Model) Created() time.Time {
	created, _ := time.Parse(time.RFC3339, m.data.CreatedAt)
	return created
}

// Aliases returns no aliases, as the API lists none
func (m *Model) Aliases() []string {
	return nil
}

// Meta returns the model's capabilities, as the API returns them
func (m *Model) Meta() map[string]json.RawMessage {
	if len(m.data.Capabilities) == 0 {
		return nil
	}
	return map[string]json.RawMessage{"capabilities": m.data.Capabilities}
}

// InputTokenLimit returns the model's context window, when it's known
func (m *Model) InputTokenLimit() *uint {
	if m.data.MaxInputTokens == 0 {
		return nil
	}
	return types.Ptr(m.data.MaxInputTokens)
}

// OutputTokenLimit returns the most tokens the model replies with, when
// it's known
func (m *Model) OutputTokenLimit() *uint {
	if m.data.MaxTokens == 0 {
		return nil
	}
	return types.Ptr(m.data.MaxTokens)
}

// Capabilities returns what the model can do: every model replies, calls
// tools, and doesn't embed, transcribe or translate, and whether it thinks,
// and understands images, is as the API says
func (m *Model) Capabilities() llm.ModelCap {
	result := llm.ModelCap{Completion: yes, Tools: yes, Embedding: no, Transcription: no, Translation: no}
	if len(m.data.Capabilities) > 0 {
		result.Thinking = types.Ptr(m.caps.Thinking.Supported)
		result.Vision = types.Ptr(m.caps.ImageInput.Supported)
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CLIENT

// ListModels returns the models, a page at a time, until they're all listed
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	var result []llm.Model
	after := ""
	for {
		models, more, err := c.listModelsPage(ctx, after, modelsPageSize)
		if err != nil {
			return nil, err
		}
		for _, data := range models.Data {
			result = append(result, newModel(c, data))
		}
		if !more || models.LastID == "" {
			return result, nil
		}
		after = models.LastID
	}
}

// GetModel returns the model with the name. Returns ErrNotFound when there's
// none.
func (c *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	if name == "" {
		return nil, llm.ErrBadParameter.With("missing model name")
	}
	var data apiModel
	if err := c.DoWithContext(ctx, nil, &data, client.OptPath("models", name)); err != nil {
		return nil, err
	}
	return newModel(c, data), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// listModelsPage returns a page of the models, after the model with the
// identifier, or from the first, when it's empty, at most the limit of them,
// and whether there are more
func (c *Client) listModelsPage(ctx context.Context, after string, limit uint) (modelList, bool, error) {
	query := url.Values{"limit": {strconv.FormatUint(uint64(limit), 10)}}
	if after != "" {
		query.Set("after_id", after)
	}
	var models modelList
	if err := c.DoWithContext(ctx, nil, &models, client.OptPath("models"), client.OptQuery(query)); err != nil {
		return modelList{}, false, err
	}
	return models, models.HasMore, nil
}

// thinks returns how the model thinks: adaptively, as it decides, within a
// budget of tokens, as older models do, or not, when it doesn't, or isn't
// known to
func (m *Model) thinks() string {
	switch {
	case m.caps.Thinking.Types.Adaptive.Supported:
		return "adaptive"
	case m.caps.Thinking.Types.Enabled.Supported, m.caps.Thinking.Supported:
		return "enabled"
	default:
		return ""
	}
}
