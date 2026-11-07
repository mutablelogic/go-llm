package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model is one of Gemini's models, satisfying the llm.Model interface
type Model struct {
	provider *Client
	data     apiModel
}

var _ llm.Model = (*Model)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// modelsPageSize is how many models are listed at once, which is the
	// most the API lists
	modelsPageSize = 1000

	// modelPrefix is the prefix of a model's name, as the API has it, which
	// the model's name doesn't have
	modelPrefix = "models/"
)

// yes and no are capabilities which a model has, or hasn't
var yes, no = types.Ptr(true), types.Ptr(false)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - MODEL

// Name returns the model's name, such as gemini-2.5-flash, without the
// API's prefix
func (m *Model) Name() string {
	return modelName(m.data.Name)
}

// Provider returns the model's provider
func (m *Model) Provider() llm.Provider {
	return m.provider
}

// Description returns the model's name for people, such as Gemini 2.5
// Flash, or what it is, when it has none
func (m *Model) Description() string {
	if m.data.DisplayName != "" {
		return m.data.DisplayName
	}
	return m.data.Description
}

// Created returns the zero time, as the API doesn't say when a model was
// created
func (m *Model) Created() time.Time {
	return time.Time{}
}

// Aliases returns the model it's a version of, when it's another, such as
// gemini-1.5-flash, for gemini-1.5-flash-001
func (m *Model) Aliases() []string {
	if base := modelName(m.data.BaseModelID); base != "" && base != m.Name() {
		return []string{base}
	}
	return nil
}

// Meta returns what the API says of the model, which isn't elsewhere: its
// version, what it is, the methods it supports, and its sampling options
func (m *Model) Meta() map[string]json.RawMessage {
	meta := make(map[string]json.RawMessage)
	add := func(key string, value any) {
		if data, err := json.Marshal(value); err == nil {
			meta[key] = data
		}
	}
	if m.data.Version != "" {
		add("version", m.data.Version)
	}
	if m.data.Description != "" && m.data.Description != m.data.DisplayName {
		add("description", m.data.Description)
	}
	if len(m.data.SupportedGenerationMethods) > 0 {
		add("supported_generation_methods", m.data.SupportedGenerationMethods)
	}
	if m.data.Temperature != nil {
		add("temperature", *m.data.Temperature)
	}
	if m.data.MaxTemperature != nil {
		add("max_temperature", *m.data.MaxTemperature)
	}
	if m.data.TopP != nil {
		add("top_p", *m.data.TopP)
	}
	if m.data.TopK != nil {
		add("top_k", *m.data.TopK)
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// InputTokenLimit returns the model's context window, when it's known
func (m *Model) InputTokenLimit() *uint {
	if m.data.InputTokenLimit == 0 {
		return nil
	}
	return types.Ptr(m.data.InputTokenLimit)
}

// OutputTokenLimit returns the most tokens the model replies with, when
// it's known
func (m *Model) OutputTokenLimit() *uint {
	if m.data.OutputTokenLimit == 0 {
		return nil
	}
	return types.Ptr(m.data.OutputTokenLimit)
}

// Capabilities returns what the model can do, of what the API says: whether
// it generates content, which is replying, or embeds it, by the methods it
// supports, whether it thinks, and takes a temperature. Models which speak,
// transcribe, or make images, which the API doesn't say, are known by their
// names: speech models, such as gemini-2.5-flash-preview-tts, and
// transcription models, which generate content, don't reply, and image
// models, such as Nano Banana, or Imagen, make images. Whether a model calls
// tools, or understands images, isn't known, as the API doesn't say.
func (m *Model) Capabilities() llm.ModelCap {
	result := llm.ModelCap{
		Completion:    types.Ptr(m.supports("generateContent")),
		Embedding:     types.Ptr(m.supports("embedContent") || m.supports("batchEmbedContents")),
		Transcription: no,
		Translation:   no,
	}
	if m.data.Thinking != nil {
		result.Thinking = types.Ptr(*m.data.Thinking)
	} else if !types.Value(result.Completion) {
		result.Thinking = no
	}
	if m.data.Temperature != nil || m.data.MaxTemperature != nil {
		result.Temperature = yes
	}

	// What it does, by its name, which the API doesn't say
	name, display := m.Name(), strings.ToLower(m.data.DisplayName)
	switch {
	case strings.Contains(name, "-tts"):
		result.Completion, result.Speech, result.Image = no, yes, no
		result.Vision, result.Audio, result.Video, result.Documents = no, no, no, no
	case strings.Contains(name, "transcribe"):
		if types.Value(result.Completion) {
			result.Completion, result.Transcription = no, yes
		}
		result.Speech, result.Image = no, no
		result.Vision, result.Audio, result.Video, result.Documents = no, yes, no, no
	case strings.Contains(name, "-image") || strings.HasPrefix(name, "imagen") || strings.Contains(display, "nano banana"):
		result.Image, result.Speech, result.Vision = yes, no, yes
	case types.Value(result.Completion):
		// Gemini's models which reply see images, hear audio, watch video,
		// and read documents
		result.Speech, result.Image = no, no
		result.Vision, result.Audio, result.Video, result.Documents = yes, yes, yes, yes
	case types.Value(result.Embedding):
		result.Speech, result.Image = no, no
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CLIENT

// ListModels returns the models, a page at a time, until they're all listed
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	var result []llm.Model
	token := ""
	for {
		models, err := c.listModelsPage(ctx, token, modelsPageSize)
		if err != nil {
			return nil, err
		}
		for _, data := range models.Models {
			result = append(result, &Model{provider: c, data: data})
		}
		if models.NextPageToken == "" {
			return result, nil
		}
		token = models.NextPageToken
	}
}

// GetModel returns the model with the name, such as gemini-2.5-flash, with
// or without the API's prefix, models/. Returns ErrNotFound when there's
// none.
func (c *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	if name = modelName(name); name == "" {
		return nil, llm.ErrBadParameter.With("missing model name")
	}
	var data apiModel
	if err := c.DoWithContext(ctx, nil, &data, client.OptPath("models", name)); err != nil {
		// A model which isn't found is ErrNotFound, with the API's error
		if status, ok := llm.HTTPStatus(err); ok && status == http.StatusNotFound {
			return nil, fmt.Errorf("%w: %w", llm.ErrNotFound.Withf("model %q", name), err)
		}
		return nil, err
	}
	return &Model{provider: c, data: data}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// listModelsPage returns a page of the models, from the page with the
// token, or the first, when it's empty, at most the limit of them, and the
// token of the next page, which is empty, when there are no more
func (c *Client) listModelsPage(ctx context.Context, token string, limit uint) (modelList, error) {
	query := url.Values{"pageSize": {strconv.FormatUint(uint64(limit), 10)}}
	if token != "" {
		query.Set("pageToken", token)
	}
	var models modelList
	if err := c.DoWithContext(ctx, nil, &models, client.OptPath("models"), client.OptQuery(query)); err != nil {
		return modelList{}, err
	}
	return models, nil
}

// supports returns whether the model supports the method, such as
// generateContent
func (m *Model) supports(method string) bool {
	return slices.Contains(m.data.SupportedGenerationMethods, method)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// modelName returns the name of a model, without the API's prefix, models/
func modelName(name string) string {
	return strings.TrimPrefix(strings.TrimSpace(name), modelPrefix)
}
