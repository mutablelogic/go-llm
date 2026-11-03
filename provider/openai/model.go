package openai

import (
	"context"
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model represents a single model, satisfying the llm.Model interface.
type Model struct {
	provider *Client
	data     apiModel
}

var _ llm.Model = (*Model)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Fields which OpenAI-compatible servers use for a model's context length,
// in order of preference: "context_length" (OpenRouter, LM Studio),
// "max_model_len" (vLLM), "context_window" (DeepSeek) and
// "max_context_length" (Mistral)
var contextLengthFields = []string{"context_length", "max_model_len", "context_window", "max_context_length"}

// Fields which OpenAI-compatible servers use for a model's maximum number of
// output tokens: "max_output_tokens" (DeepSeek)
var outputTokensFields = []string{"max_output_tokens"}

// Fields which OpenAI-compatible servers use for a model's description, in
// order of preference: "description" (OpenRouter), and its display name,
// "name" (DeepSeek)
var descriptionFields = []string{"description", "name"}

// openAIHost is the host of OpenAI's API, whose models use the table of
// limits, deepSeekHost the host of DeepSeek's, and mistralHost the host of
// Mistral's
const (
	openAIHost   = "api.openai.com"
	deepSeekHost = "api.deepseek.com"
	mistralHost  = "api.mistral.ai"
)

// modelCapabilities are the capabilities of models which servers don't
// report, by a regular expression of their names, which are set on those
// the server reports: Mistral's embedding models, which it reports no
// capabilities of
var modelCapabilities = []struct {
	re  *regexp.Regexp
	set func(*llm.ModelCap)
}{
	{regexp.MustCompile(`^(mistral|codestral)-embed`), func(c *llm.ModelCap) { c.Embedding = types.Ptr(true) }},
}

// modelLimits are the context window and maximum output tokens of a model
type modelLimits struct {
	input, output uint
}

// reDateSuffix matches the date at the end of a dated version of a model's
// name, such as "-2024-08-06" or "-0125"
var reDateSuffix = regexp.MustCompile(`-(\d{4}-\d{2}-\d{2}|\d{4})$`)

// openAIModelLimits are the token limits of OpenAI's models, which its API
// doesn't return, by model name. A dated version of a model, such as
// "gpt-4o-2024-08-06", uses the limits of the model. Other models, such as
// "gpt-4o-transcribe", or newer models, have no limits until they're added.
// TODO: these values are from memory, and should be checked against OpenAI's
// model documentation.
var openAIModelLimits = map[string]modelLimits{
	"gpt-5":         {400000, 128000},
	"gpt-5-mini":    {400000, 128000},
	"gpt-5-nano":    {400000, 128000},
	"gpt-4.1":       {1047576, 32768},
	"gpt-4.1-mini":  {1047576, 32768},
	"gpt-4.1-nano":  {1047576, 32768},
	"gpt-4o":        {128000, 16384},
	"gpt-4o-mini":   {128000, 16384},
	"gpt-4-turbo":   {128000, 4096},
	"gpt-3.5-turbo": {16385, 4096},
	"o1":            {200000, 100000},
	"o1-mini":       {128000, 65536},
	"o3":            {200000, 100000},
	"o3-mini":       {200000, 100000},
	"o4-mini":       {200000, 100000},
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the model name, e.g. "gpt-4o"
func (m *Model) Name() string {
	return m.data.ID
}

// Provider returns the client this model came from.
func (m *Model) Provider() llm.Provider {
	return m.provider
}

// Description returns the model's description, or its display name, which
// some OpenAI-compatible servers return, such as OpenRouter and DeepSeek.
// OpenAI itself doesn't.
func (m *Model) Description() string {
	for _, field := range descriptionFields {
		var description string
		if raw, exists := m.data.Fields[field]; exists && json.Unmarshal(raw, &description) == nil && description != "" {
			return description
		}
	}
	return ""
}

// Created returns when the model was created, or the zero time if the server
// doesn't say.
func (m *Model) Created() time.Time {
	if m.data.Created == 0 {
		return time.Time{}
	}
	return time.Unix(m.data.Created, 0)
}

// Aliases returns alternate names for the model, which some
// OpenAI-compatible servers return, such as Mistral, whose "-latest" names
// are aliases of a dated model. OpenAI itself doesn't.
func (m *Model) Aliases() []string {
	var aliases []string
	if raw, exists := m.data.Fields["aliases"]; exists && json.Unmarshal(raw, &aliases) == nil {
		return slices.DeleteFunc(aliases, func(alias string) bool { return alias == "" || alias == m.data.ID })
	}
	return nil
}

// Meta returns provider-specific metadata: all the model's fields, apart from
// its name and creation time.
func (m *Model) Meta() map[string]json.RawMessage {
	meta := maps.Clone(m.data.Fields)
	delete(meta, "id")
	delete(meta, "created")
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// InputTokenLimit returns the model's context length, for servers which
// return one. OpenAI itself doesn't.
func (m *Model) InputTokenLimit() *uint {
	if limit := m.tokenLimit(contextLengthFields); limit != nil {
		return limit
	} else if limits := m.openAILimits(); limits != nil {
		return &limits.input
	}
	return nil
}

// Capabilities returns what the server reports the model can do, which
// Mistral does, as its capabilities, or none of them, with those which are
// known by the model's name, such as that Mistral's embedding models embed.
// Mistral doesn't say which reasoning efforts a model which reasons accepts,
// so they're nil, or empty when it doesn't reason, and it doesn't report
// embedding or translation. OpenAI itself reports none.
func (m *Model) Capabilities() llm.ModelCap {
	result := m.reportedCapabilities()
	for _, model := range modelCapabilities {
		if model.re.MatchString(m.data.ID) {
			model.set(&result)
		}
	}
	return result
}

// reportedCapabilities returns what the server reports the model can do, as
// Mistral's capabilities, or none of them
func (m *Model) reportedCapabilities() llm.ModelCap {
	var capabilities struct {
		Completion    *bool `json:"completion_chat"`
		Tools         *bool `json:"function_calling"`
		Reasoning     *bool `json:"reasoning"`
		Vision        *bool `json:"vision"`
		OCR           *bool `json:"ocr"`
		Transcription *bool `json:"audio_transcription"`
	}
	raw, exists := m.data.Fields["capabilities"]
	if !exists || json.Unmarshal(raw, &capabilities) != nil {
		return llm.ModelCap{}
	}
	result := llm.ModelCap{
		Completion:    capabilities.Completion,
		Tools:         capabilities.Tools,
		Vision:        capabilities.Vision,
		OCR:           capabilities.OCR,
		Transcription: capabilities.Transcription,
	}
	if capabilities.Reasoning != nil && !*capabilities.Reasoning {
		result.Thinking = []string{}
	}
	return result
}

// OutputTokenLimit returns the model's maximum number of output tokens, for
// servers which return one. OpenAI itself doesn't.
func (m *Model) OutputTokenLimit() *uint {
	if limit := m.tokenLimit(outputTokensFields); limit != nil {
		return limit
	} else if limits := m.openAILimits(); limits != nil {
		return &limits.output
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// openAILimits returns the token limits of one of OpenAI's models, from the
// table of limits, by its name, or its name without a date suffix. Returns
// nil if the model isn't one of OpenAI's, or isn't in the table. OpenAI's API
// doesn't return the limits.
func (m *Model) openAILimits() *modelLimits {
	if m.provider == nil || m.provider.url == nil || !strings.EqualFold(m.provider.url.Hostname(), openAIHost) {
		return nil
	}
	for _, name := range []string{m.data.ID, reDateSuffix.ReplaceAllString(m.data.ID, "")} {
		if limits, exists := openAIModelLimits[name]; exists {
			return &limits
		}
	}
	return nil
}

// tokenLimit returns the first of the fields which is a positive number
func (m *Model) tokenLimit(fields []string) *uint {
	for _, field := range fields {
		raw, exists := m.data.Fields[field]
		if !exists {
			continue
		}
		var n uint
		if err := json.Unmarshal(raw, &n); err == nil && n > 0 {
			return &n
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER METHODS

// ListModels returns the models available from the provider.
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	models, err := c.listModels(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]llm.Model, len(models))
	for i, data := range models {
		result[i] = &Model{provider: c, data: data}
	}
	return result, nil
}

// GetModel returns the model with the given name. The model is found in the
// list of models, rather than retrieved by name, as not every OpenAI-compatible
// server can retrieve a single model, and model names can contain "/".
func (c *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	models, err := c.listModels(ctx)
	if err != nil {
		return nil, err
	}
	for _, data := range models {
		if data.ID == name {
			return &Model{provider: c, data: data}, nil
		}
	}
	return nil, llm.ErrNotFound.Withf("model %q not found", name)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// listModels returns the models from GET /models, without the models which
// have been shut down
func (c *Client) listModels(ctx context.Context) ([]apiModel, error) {
	var response listModelsResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}
	now := time.Now()
	return slices.DeleteFunc(response.Data, func(model apiModel) bool {
		return model.shutDown(now)
	}), nil
}

// shutDown reports whether the model has been shut down at the time, as its
// "shutdown_date", which OpenAI returns for models which are being retired,
// is the day or earlier. A model without a date, or whose date isn't a date,
// hasn't been shut down.
func (m apiModel) shutDown(now time.Time) bool {
	var date string
	if raw, exists := m.Fields["shutdown_date"]; !exists || json.Unmarshal(raw, &date) != nil || date == "" {
		return false
	}
	shutdown, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return false
	}
	return !now.UTC().Before(shutdown)
}
