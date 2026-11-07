package gowhisper

import (
	"context"
	"encoding/json"
	"path"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Model is a whisper.cpp model of a go-whisper server
type Model struct {
	provider *Client
	data     apiModel
}

var _ llm.Model = (*Model)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the model's id, such as "ggml-medium-q5_0"
func (m *Model) Name() string {
	return m.data.Id
}

// Provider returns the client the model came from
func (m *Model) Provider() llm.Provider {
	return m.provider
}

// Description returns the model's file, such as "ggml-medium-q5_0.bin", as
// go-whisper has no description
func (m *Model) Description() string {
	return m.data.Path
}

// Created returns when the model was downloaded
func (m *Model) Created() time.Time {
	if m.data.Created <= 0 {
		return time.Time{}
	}
	return time.Unix(m.data.Created, 0)
}

// Aliases returns the model's file, such as "ggml-medium-q5_0.bin", which
// it's downloaded by, when it's not its name
func (m *Model) Aliases() []string {
	if m.data.Path != "" && m.data.Path != m.data.Id {
		return []string{m.data.Path}
	}
	return nil
}

// Meta returns the model's file, and who owns it
func (m *Model) Meta() map[string]json.RawMessage {
	meta := make(map[string]json.RawMessage, 2)
	for key, value := range map[string]string{"path": m.data.Path, "owned_by": m.data.OwnedBy} {
		if value == "" {
			continue
		} else if data, err := json.Marshal(value); err == nil {
			meta[key] = data
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// InputTokenLimit isn't known, as a whisper model transcribes audio
func (m *Model) InputTokenLimit() *uint {
	return nil
}

// OutputTokenLimit isn't known, as a whisper model transcribes audio
func (m *Model) OutputTokenLimit() *uint {
	return nil
}

// Capabilities returns what the model can do, which is transcribe speech,
// and translate it into English, unless it's an English-only model, whose
// name has ".en", such as "ggml-tiny.en", and nothing else
func (m *Model) Capabilities() llm.ModelCap {
	no := types.Ptr(false)
	return llm.ModelCap{
		Chat:          no,
		Tools:         no,
		Thinking:      no,
		ImageInput:    no,
		OCR:           no,
		Transcription: types.Ptr(true),
		Translation:   types.Ptr(!strings.Contains(m.data.Id, ".en")),
		Embedding:     no,
	}
}

///////////////////////////////////////////////////////////////////////////////
// PROVIDER METHODS

// ListModels returns go-whisper's own models, which it runs, rather than
// those of the services it forwards to, such as OpenAI and ElevenLabs
func (c *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	models, err := c.listModels(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]llm.Model, 0, len(models))
	for _, data := range models {
		if data.OwnedBy == ownedBy {
			result = append(result, &Model{provider: c, data: data})
		}
	}
	return result, nil
}

// GetModel returns the model with the name, which is its id, such as
// "ggml-medium-q5_0", or its file, such as "ggml-medium-q5_0.bin". Returns
// ErrNotFound when it isn't one of go-whisper's own models.
func (c *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	id := modelID(name)
	if id == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	}
	var data apiModel
	if err := c.DoWithContext(ctx, nil, &data, client.OptPath("model", id)); err != nil {
		return nil, err
	} else if data.OwnedBy != ownedBy {
		return nil, llm.ErrNotFound.Withf("model %q", name)
	}
	return &Model{provider: c, data: data}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// listModels returns all the models of the server, including those of the
// services it forwards to
func (c *Client) listModels(ctx context.Context) ([]apiModel, error) {
	var models []apiModel
	if err := c.DoWithContext(ctx, nil, &models, client.OptPath("model")); err != nil {
		return nil, err
	}
	return models, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// modelID returns the id of a model, by its name, which is its id, or its
// file, which can have a directory, such as "english/ggml-tiny.en.bin",
// whose id is its base name, as go-whisper makes it: in lower case, without
// its extension, and with each character which isn't a letter, a digit, a
// full stop or a hyphen as an underscore
func modelID(name string) string {
	name = strings.TrimSpace(name)
	if !strings.HasSuffix(strings.ToLower(name), modelExt) && !strings.Contains(name, "/") {
		return name
	}
	base := strings.ToLower(path.Base(name))
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-':
			return r
		default:
			return '_'
		}
	}, strings.TrimSuffix(base, path.Ext(base)))
}

// modelFile returns the file of a model, by its name, which is its file, or
// its id, with the extension
func modelFile(name string) string {
	if name = strings.TrimSpace(name); name == "" || strings.HasSuffix(name, modelExt) {
		return name
	}
	return name + modelExt
}
