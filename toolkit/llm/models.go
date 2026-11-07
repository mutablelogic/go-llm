package llm

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
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
	Capabilities []string `json:"capabilities,omitempty" help:"Capabilities which the models must have, every one of them: completion, tools, thinking, vision, ocr, transcription, translation, embedding, speech or image, or none, for every model" optional:""`
	Provider     string   `json:"provider,omitempty" help:"Provider whose models are listed, by its name, such as ollama, or every provider's, when it's omitted" optional:""`
}

// modelsOutput is the models, and how many there are
type modelsOutput struct {
	Count  int           `json:"count"`
	Models []modelOutput `json:"models"`
}

// modelOutput is a model: its name, by which it's given to a tool, what it
// is, what it can do, and how many tokens it takes, and returns, when that's
// known
type modelOutput struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	InputTokens  *uint    `json:"input_tokens,omitempty"`
	OutputTokens *uint    `json:"output_tokens,omitempty"`
}

var _ gollm.Tool = (*listModels)(nil)

// ability is what a model with each capability can do, as it's said
var ability = map[string]string{
	"completion":    "reply",
	"tools":         "call tools",
	"thinking":      "think",
	"vision":        "see images",
	"ocr":           "read documents",
	"transcription": "transcribe speech",
	"translation":   "translate speech",
	"embedding":     "embed text",
	"speech":        "speak",
	"image":         "make images",
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*listModels) Name() string {
	return "models"
}

func (*listModels) Description() string {
	return "Lists the models which have every one of the capabilities, such as ocr, transcription, speech, vision, or image, which is making images, with their names, which are given to the tools which ask them, such as ocr, transcribe and speak, or to an agent, so the right model for the task is chosen. A capability which isn't known for a model isn't listed."
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
	result := modelsOutput{Count: len(list), Models: make([]modelOutput, 0, len(list))}
	for _, model := range list {
		result.Models = append(result.Models, modelOutput{
			Name:         m.Name(model),
			Description:  model.Description(),
			Capabilities: model.Capabilities().Names(),
			InputTokens:  model.InputTokenLimit(),
			OutputTokens: model.OutputTokenLimit(),
		})
	}
	return result, nil
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
