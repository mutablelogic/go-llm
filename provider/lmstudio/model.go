package lmstudio

import (
	"context"
	"encoding/json"
	"maps"
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

// Model is a model of an LM Studio server, which is stored on it, and may be
// loaded, in memory
type Model struct {
	provider *Client
	data     apiModel
}

var _ llm.Model = (*Model)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the model's name, which is its key, such as
// "google/gemma-4-12b-qat"
func (m *Model) Name() string {
	return m.data.Key
}

// Provider returns the client the model is from
func (m *Model) Provider() llm.Provider {
	return m.provider
}

// Description returns the model's description, or its display name, such
// as "Gemma 4 12B QAT", when it has none
func (m *Model) Description() string {
	if description := strings.TrimSpace(types.Value(m.data.Description)); description != "" {
		return description
	}
	return m.data.DisplayName
}

// Created returns the zero time, as LM Studio doesn't say when a model was
// made
func (m *Model) Created() time.Time {
	return time.Time{}
}

// Aliases returns no aliases, as LM Studio has none
func (m *Model) Aliases() []string {
	return nil
}

// Meta returns what LM Studio says of the model, such as its publisher,
// architecture, quantization, size, and the instances of it which are
// loaded, apart from its key
func (m *Model) Meta() map[string]json.RawMessage {
	meta := maps.Clone(m.data.Fields)
	delete(meta, "key")
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// InputTokenLimit returns the model's context length, which is the one it
// was loaded with, when it's loaded, as LM Studio sets it as it's loaded,
// and otherwise the most it can have
func (m *Model) InputTokenLimit() *uint {
	var limit uint
	for _, instance := range m.data.LoadedInstances {
		if n := instance.Config.ContextLength; n > 0 && (limit == 0 || n < limit) {
			limit = n
		}
	}
	if limit == 0 {
		limit = m.data.MaxContextLength
	}
	if limit == 0 {
		return nil
	}
	return &limit
}

// OutputTokenLimit returns nil, as LM Studio doesn't say
func (m *Model) OutputTokenLimit() *uint {
	return nil
}

// Capabilities returns what the model can do: a language model replies, and
// calls tools, when it was trained to, sees images, when it has vision,
// thinks, when it reasons, and recognizes text, when it's an OCR model, such
// as olmOCR, and an embedding model embeds. A model which
// replies is only sent images, as the chat completions API which it replies
// with has no audio, video, or documents.
func (m *Model) Capabilities() llm.ModelCap {
	yes, no := types.Ptr(true), types.Ptr(false)
	switch m.data.Type {
	case typeLLM:
		result := llm.ModelCap{Chat: yes, Temperature: yes, Embedding: no, AudioInput: no, VideoInput: no, DocumentInput: no}
		if c := m.data.Capabilities; c != nil {
			result.Tools, result.ImageInput = types.Ptr(c.ToolUse), types.Ptr(c.Vision)
			result.Thinking = types.Ptr(c.Reasoning.thinks())
		}

		// A model recognizes text when it's an OCR model, by its name, and
		// doesn't when it doesn't see images, and isn't known otherwise, as
		// a model which sees images can read text in them, but not as well
		switch {
		case findOCRModel(m.data.Key) != nil:
			result.OCR, result.ImageInput = yes, yes
		case result.ImageInput != nil && !*result.ImageInput:
			result.OCR = no
		}
		return result
	case typeEmbedding:
		return llm.ModelCap{Chat: no, Tools: no, Thinking: no, ImageInput: no, Embedding: yes}
	default:
		return llm.ModelCap{}
	}
}

// Loaded returns whether the model is loaded, in memory
func (m *Model) Loaded() bool {
	return len(m.data.LoadedInstances) > 0
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER METHODS

// ListModels returns the models which are stored on the server
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

// GetModel returns the model with the name, which is found in the list of
// models, as LM Studio doesn't get a model by its name
func (c *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	models, err := c.listModels(ctx)
	if err != nil {
		return nil, err
	}
	for _, data := range models {
		if data.Key == name {
			return &Model{provider: c, data: data}, nil
		}
	}
	return nil, llm.ErrNotFound.Withf("model %q not found", name)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// listModels returns the models, as LM Studio's REST API lists them
func (c *Client) listModels(ctx context.Context) ([]apiModel, error) {
	var response listModelsResponse
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models")); err != nil {
		return nil, err
	}
	return response.Models, nil
}

///////////////////////////////////////////////////////////////////////////////
// REASONING

// thinks returns whether a model with the reasoning reasons, which it does
// unless its only setting is "off"
func (r *reasoning) thinks() bool {
	return r != nil && (len(r.Allowed) == 0 || slices.ContainsFunc(r.Allowed, func(option string) bool { return option != reasoningOff }))
}

// efforts returns the efforts the reasoning allows, such as "low", "medium"
// and "high", which a model whose reasoning is only "off" and "on" hasn't
func (r *reasoning) efforts() []string {
	if r == nil {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(r.Allowed), func(option string) bool {
		return !slices.Contains(llm.Efforts, option)
	})
}

// reasoningOpts returns the options, with the thinking, and its effort, as
// LM Studio's OpenAI-compatible API is asked for them, as its
// reasoning_effort, for the model's reasoning:
//   - thinking off is "none", when the model can turn it off, or its own;
//   - an effort is the nearest the model has, or, for a model which only
//     turns its reasoning off and on, which an effort doesn't change,
//     "medium", when it doesn't think unless it's asked, or its own;
//   - thinking on, without an effort, is "medium", or the nearest, when it
//     doesn't think unless it's asked, or its own.
//
// Thinking is ignored by a model which doesn't reason, and an effort is
// passed on.
func (m *Model) reasoningOpts(opts []llm.Opt) ([]llm.Opt, error) {
	var r *reasoning
	if m.data.Capabilities != nil {
		r = m.data.Capabilities.Reasoning
	}
	if r == nil {
		return opts, nil
	}
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}
	thinking, err := llm.GetOpt[bool](o, llm.ThinkingKey)
	asked := err == nil
	effort, _ := llm.GetOpt[string](o, llm.ThinkingEffortKey)
	efforts := r.efforts()
	switch {
	case asked && !thinking && slices.Contains(r.Allowed, reasoningOff):
		effort = "none"
	case asked && !thinking:
		effort = ""
	case effort == "" && (!asked || r.Default != reasoningOff):
		// Its own reasoning, unless it's asked to think, and doesn't
	case effort == "":
		effort = llm.NearestEffort("medium", efforts...)
	case len(efforts) > 0:
		effort = llm.NearestEffort(effort, efforts...)
	case r.Default == reasoningOff:
		effort = "medium"
	default:
		effort = ""
	}
	return append(slices.Clone(opts), llm.WithOpt(llm.ThinkingEffortKey, effort)), nil
}
