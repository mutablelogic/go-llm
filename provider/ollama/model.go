package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model represents a single Ollama model, satisfying the llm.Model interface.
type Model struct {
	provider *Client
	data     apiModel
}

var _ llm.Model = (*Model)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// showConcurrency is the number of models shown at a time when models are
// listed
const showConcurrency = 4

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the model name, e.g. "llama3.2:latest"
func (m *Model) Name() string {
	return m.data.Name
}

// Provider returns the client this model came from.
func (m *Model) Provider() llm.Provider {
	return m.provider
}

// Description summarises the model's family and size, e.g.
// "llama - 3B params, Q4_K_M quantization". Ollama has no separate
// human-readable description field, so this is derived from Details.
func (m *Model) Description() string {
	d := m.data.Details
	var parts []string
	if d.Family != "" {
		parts = append(parts, d.Family)
	}
	if d.ParameterSize != "" {
		parts = append(parts, fmt.Sprintf("%s params", d.ParameterSize))
	}
	if d.QuantizationLevel != "" {
		parts = append(parts, fmt.Sprintf("%s quantization", d.QuantizationLevel))
	}
	return strings.Join(parts, ", ")
}

// Created returns when the model was last pulled or modified locally -
// Ollama has no separate "created" timestamp.
func (m *Model) Created() time.Time {
	return m.data.ModifiedAt
}

// Aliases returns alternate names for the model, which is its name without
// its tag, when its tag is the default, "latest", such as "llama3.2" for
// "llama3.2:latest", as Ollama uses the default tag when there's none
func (m *Model) Aliases() []string {
	if name, ok := strings.CutSuffix(m.data.Name, ":"+defaultTag); ok && name != "" {
		return []string{name}
	}
	return nil
}

// Meta returns provider-specific metadata: the model's "details" object,
// plus "model_info" if the API returned it (only for a single model
// fetched via GetModel, not for ListModels' summary entries).
func (m *Model) Meta() map[string]json.RawMessage {
	meta := make(map[string]json.RawMessage, len(m.data.Info)+1)
	if data, err := json.Marshal(m.data.Details); err == nil {
		meta["details"] = data
	}
	maps.Copy(meta, m.data.Info)
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// InputTokenLimit returns the model's context length, if Ollama reported
// one in model_info. The context length of the model's family, such as
// "llama.context_length", is preferred, as model_info can have more than one,
// such as for a vision model's projector. Otherwise any context length is
// used, in order of key.
func (m *Model) InputTokenLimit() *uint {
	if family := m.data.Details.Family; family != "" {
		if n := positiveUint(m.data.Info[family+".context_length"]); n != nil {
			return n
		}
	}
	for _, key := range slices.Sorted(maps.Keys(m.data.Info)) {
		if key == "context_length" || strings.HasSuffix(key, ".context_length") {
			if n := positiveUint(m.data.Info[key]); n != nil {
				return n
			}
		}
	}
	return nil
}

// OutputTokenLimit is not exposed by Ollama's API.
func (m *Model) OutputTokenLimit() *uint {
	return nil
}

// Capabilities returns what Ollama reports the model can do, which is none
// of them when it didn't report the model's capabilities, as when it
// couldn't be shown. It doesn't report OCR, transcription or translation.
func (m *Model) Capabilities() llm.ModelCap {
	capabilities := m.data.Capabilities
	if len(capabilities) == 0 {
		return llm.ModelCap{}
	}
	has := func(capability string) *bool {
		return types.Ptr(slices.Contains(capabilities, capability))
	}
	return llm.ModelCap{
		Completion: has("completion"),
		Tools:      has("tools"),
		Thinking:   has("thinking"),
		Vision:     has("vision"),
		Embedding:  has("embedding"),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER METHODS

// ListModels returns the list of models available in the local Ollama
// registry.
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	var response listModelsResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("tags")); err != nil {
		return nil, err
	}

	// The list doesn't include each model's model_info, such as its context
	// length, so show each model, a few at a time. A model which can't be
	// shown is listed without it.
	var wg sync.WaitGroup
	sem := make(chan struct{}, showConcurrency)
	result := make([]llm.Model, len(response.Models))
	for i, data := range response.Models {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			if shown, err := c.show(ctx, data.Name); err == nil {
				data.Details = shown.Details
				data.Capabilities = shown.Capabilities
				data.Info = shown.Info
			}
			result[i] = &Model{provider: c, data: data}
		})
	}
	wg.Wait()
	return result, nil
}

// GetModel returns details of the model with the given name.
func (c *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	data, err := c.show(ctx, name)
	if err != nil {
		return nil, err
	}
	if data.Name == "" {
		data.Name = name
	}
	return &Model{provider: c, data: *data}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// show returns the details of the model with the name, from POST /api/show
func (c *Client) show(ctx context.Context, name string) (*apiModel, error) {
	req, err := client.NewJSONRequest(map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	var data apiModel
	if err := c.DoWithContext(ctx, req, &data, client.OptPath("show")); err != nil {
		return nil, err
	}
	return &data, nil
}

// positiveUint returns the JSON value as a number, or nil if it isn't a
// positive whole number
func positiveUint(raw json.RawMessage) *uint {
	var n uint
	if err := json.Unmarshal(raw, &n); err != nil || n == 0 {
		return nil
	}
	return &n
}
