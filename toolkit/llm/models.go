package llm

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// listModels is the tool which lists the models which have capabilities, such
// as ocr, so the model which calls it can choose the one to give another
// tool, such as ocr, or an agent
type listModels struct {
	toolkit.Base
	models ModelsFunc
}

// modelsInput is the input of the models tool
type modelsInput struct {
	Capabilities []string `json:"capabilities,omitempty" help:"Capabilities which the models must have, every one of them: chat, tools, thinking, image_input, audio_input, video_input, document_input, image_output, speech_output, embedding, ocr, transcription or translation, or none, for every model" optional:""`
	Provider     string   `json:"provider,omitempty" help:"Provider whose models are listed, by its name, such as ollama, or every provider's, when it's omitted" optional:""`
	Model        string   `json:"model,omitempty" help:"The one model which is listed, by its name, such as ollama:phonon:latest, or every model, when it's omitted" optional:""`
	VoicesNext   string   `json:"voices_next,omitempty" help:"The next page of the model's voices, which is the opaque voices_next of the page before, passed back as it is, with the model, and the voice, when there's one" optional:""`
	Voice        string   `json:"voice,omitempty" help:"Part of the name, or id, of the model's voices to find, ignoring case, such as matilda, with the model" optional:""`
}

// modelsOutput is the models, and how many there are
type modelsOutput struct {
	Count  int           `json:"count"`
	Models []modelOutput `json:"models"`
	Note   string        `json:"note,omitempty"`
}

// modelOutput is a model: its name, by which it's given to a tool, what it
// is, what it can do, and how many tokens it takes, and returns, when that's
// known, and, when models which speak are asked for, the voices it speaks
// in, when its provider says
type modelOutput struct {
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Capabilities []string      `json:"capabilities,omitempty"`
	InputTokens  *uint         `json:"input_tokens,omitempty"`
	OutputTokens *uint         `json:"output_tokens,omitempty"`
	Voices       []gollm.Voice `json:"voices,omitempty"`
	VoicesCount  *uint         `json:"voices_count,omitempty"`
	VoicesNext   string        `json:"voices_next,omitempty"`
}

var _ gollm.Tool = (*listModels)(nil)

// voicesPage is the most voices of a model which are listed at once, as
// there can be many, such as an ElevenLabs account's
const voicesPage = 25

// ability is what a model with each capability can do, as it's said
var ability = map[string]string{
	"chat":           "reply",
	"tools":          "call tools",
	"thinking":       "think",
	"image_input":    "see images",
	"audio_input":    "hear audio",
	"video_input":    "watch video",
	"document_input": "read documents",
	"image_output":   "make images",
	"speech_output":  "speak",
	"embedding":      "embed text",
	"ocr":            "recognize text",
	"transcription":  "transcribe speech",
	"translation":    "translate speech",
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*listModels) Name() string {
	return "models"
}

func (*listModels) Description() string {
	return "Lists the models which have every one of the capabilities, such as ocr, transcription, speech_output, image_input, which is seeing images, or image_output, which is making them, with their names, which are given to the tools which ask them, such as ocr, transcribe and speak, or to an agent, so the right model for the task is chosen. A capability which isn't known for a model isn't listed. With speech_output, and a model, the model has the voices it speaks in, when its provider says, each with its id, and its title, either of which the speak tool is given, a page at a time, with voices_count, which is how many there are: when there are more, its voices_next, which is opaque, is given back, as it is, with the model, for the next page, and the last page has none. A voice is found by part of its name with voice, and the model."
}

func (*listModels) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[modelsInput]()
}

// Run returns the models, of every provider, or the one, which are known
// to have every one of the capabilities
func (t *listModels) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req modelsInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, gollm.ErrBadParameter.With(err.Error())
		}
	}
	// The capabilities are checked first, so the model is told one isn't,
	// rather than that a provider can't be reached
	if err := gollm.ValidCapabilities(req.Capabilities...); err != nil {
		return nil, err
	} else if (req.VoicesNext != "" || strings.TrimSpace(req.Voice) != "") && strings.TrimSpace(req.Model) == "" {
		return nil, gollm.ErrBadParameter.With("voices_next and voice are a model's voices, so the model is needed too")
	}
	m := t.models()
	if m == nil {
		return nil, gollm.ErrServiceUnavailable.With("no models yet")
	}
	var providers []string
	if provider := strings.TrimSpace(req.Provider); provider != "" {
		providers = append(providers, provider)
	}

	// The models of the providers which can be listed
	if len(req.Capabilities) > 0 {
		toolkit.Logf(ctx, "Looking for models which can %s", abilities(req.Capabilities))
	} else {
		toolkit.Logf(ctx, "Looking at the models")
	}
	list, err := m.ListModels(ctx, providers...)
	if err != nil && len(list) == 0 {
		return nil, err
	}
	list, err = gollm.WithCapabilities(list, req.Capabilities...)
	if err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(req.Model); name != "" {
		list = slices.DeleteFunc(list, func(model gollm.Model) bool {
			return m.Name(model) != name && model.Name() != name
		})
	}
	// With the voices of a model which speaks, when it's asked for, with
	// speech_output, or its next page is, or voices like a name, a page at a
	// time, as there can be many, and models of a provider, such as
	// ElevenLabs, have the same voices. A model whose voices can't be listed
	// is listed without them, apart from one whose voices are asked for by
	// their page, or a name, which is said to be.
	one := strings.TrimSpace(req.Model) != ""
	search := strings.ToLower(strings.TrimSpace(req.Voice))
	asked := req.VoicesNext != "" || search != ""
	voices := one && (slices.Contains(req.Capabilities, "speech_output") || asked)
	result := modelsOutput{Count: len(list), Models: make([]modelOutput, 0, len(list))}
	for _, model := range list {
		output := modelOutput{
			Name:         m.Name(model),
			Description:  model.Description(),
			Capabilities: model.Capabilities().Names(),
			InputTokens:  model.InputTokenLimit(),
			OutputTokens: model.OutputTokenLimit(),
		}
		if voices {
			list, err := t.voices(ctx, m, model, search, req.VoicesNext)
			if err != nil && asked {
				return nil, err
			} else if err == nil {
				output.Voices, output.VoicesCount, output.VoicesNext = list.Body, types.Ptr(list.Count), list.Next
			}
		}
		result.Models = append(result.Models, output)
	}
	if !one && slices.Contains(req.Capabilities, "speech_output") && len(list) > 0 {
		result.Note = "Give one of the models, as model, with speech_output, to list its voices"
	}
	return result, nil
}

// voices returns the model's voices, or those whose title, or id, has the
// search, ignoring case, a page at a time, from the page the token says,
// with how many there are, and the token of the next, when there are more
func (t *listModels) voices(ctx context.Context, m Models, model gollm.Model, search, token string) (*gollm.VoiceList, error) {
	opts := []gollm.Opt{gollm.WithLimit(voicesPage)}
	if token != "" {
		opts = append(opts, gollm.WithPageToken(token))
	}
	if search == "" {
		return m.ListVoices(ctx, model, opts...)
	}
	all, err := m.ListVoices(ctx, model)
	if err != nil {
		return nil, err
	}
	return gollm.PagedVoices(gollm.FindVoices(all.Body, search), opts...)
}

// abilities returns what models with the capabilities can do, as it's said,
// such as "read documents and speak"
func abilities(capabilities []string) string {
	var result []string
	for _, capability := range capabilities {
		name := strings.ToLower(strings.TrimSpace(capability))
		if phrase, ok := ability[name]; ok {
			result = append(result, phrase)
		} else {
			result = append(result, name)
		}
	}
	switch len(result) {
	case 0:
		return "do anything"
	case 1:
		return result[0]
	}
	return strings.Join(result[:len(result)-1], ", ") + " and " + result[len(result)-1]
}
