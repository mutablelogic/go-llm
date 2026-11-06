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
	Capabilities []string `json:"capabilities,omitempty" help:"Capabilities which the models must have, every one of them: completion, tools, thinking, vision, ocr, transcription, translation, embedding or speech, or none, for every model" optional:""`
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

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*listModels) Name() string {
	return "models"
}

func (*listModels) Description() string {
	return "Lists the models which have every one of the capabilities, such as ocr, transcription, speech or vision, with their names, which are given to the tools which ask them, such as ocr, transcribe and speak, or to an agent, so the right model for the task is chosen. A capability which isn't known for a model isn't listed."
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
		toolkit.Logf(ctx, "Looking for models with %s", strings.Join(req.Capabilities, ", "))
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
