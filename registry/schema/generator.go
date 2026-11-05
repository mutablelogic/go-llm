package schema

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// GeneratorMeta is how a model is asked: the model, the system prompt, the
// tools it can call, the options, and the schema of its reply, which agents,
// sessions and requests to ask a model have
type GeneratorMeta struct {
	Model  *string                    `json:"model,omitempty" help:"Model, by name, or by provider and name" example:"ollama:llama3.2:latest" optional:""`
	System *string                    `json:"system,omitempty" help:"System prompt, sent before the messages" example:"Be brief" optional:""`
	Tools  []string                   `json:"tools,omitzero" help:"Tools the model can call, by name, by namespace and name, all the tools in a namespace, such as weatherapi:*, or * for all the tools" example:"[\"weatherapi:current\"]" optional:""`
	Opt    map[string]json.RawMessage `json:"opt,omitzero" help:"Options, by key, such as temperature=0.7, which the provider decodes" example:"{\"temperature\":0.7}" optional:""`
	Schema json.RawMessage            `json:"schema,omitempty" help:"JSON schema of the reply, which the model replies with, or text when omitted" example:"{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}}}" optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (g GeneratorMeta) String() string {
	return types.Stringify(g)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// JSONSchema returns the schema of the reply, or nil when the reply is text.
// Returns ErrBadParameter when it isn't a JSON schema.
func (g GeneratorMeta) JSONSchema() (*jsonschema.Schema, error) {
	data := bytes.TrimSpace(g.Schema)
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	result, err := jsonschema.FromJSON(json.RawMessage(data))
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid schema: %v", err)
	}
	return result, nil
}

// Opts returns the system prompt, and the options, for the provider to
// decode. A tool choice of "none" isn't an option, as the model is sent no
// tools instead, so a provider which can't choose tools needn't. Returns
// ErrBadParameter when an option is one which there's a field for, which is
// the system prompt, and the schema.
func (g GeneratorMeta) Opts(system string) ([]llm.Opt, error) {
	opts := make([]llm.Opt, 0, len(g.Opt)+1)
	if system = strings.TrimSpace(system); system != "" {
		opts = append(opts, llm.WithSystemPrompt(system))
	}
	for key, value := range g.Opt {
		switch strings.TrimSpace(key) {
		case llm.SystemPromptKey:
			return nil, llm.ErrBadParameter.Withf("invalid option %q: set the system prompt as system", key)
		case llm.JSONSchemaKey:
			return nil, llm.ErrBadParameter.Withf("invalid option %q: set the schema of the reply as schema", key)
		case llm.ToolChoiceKey:
			var choice string
			if json.Unmarshal(value, &choice) == nil && choice == "none" {
				continue
			}
		}
		opts = append(opts, llm.WithOpt(key, value))
	}
	return opts, nil
}

// IsZero returns true when the settings set nothing, so as an update, they
// change nothing
func (s GeneratorMeta) IsZero() bool {
	return s.Model == nil && s.System == nil && s.Tools == nil && len(s.Opt) == 0 && len(bytes.TrimSpace(s.Schema)) == 0
}

// Merge returns the settings, changed by those of the update: its model, and
// its system prompt, when they're set, which empty clears; its tools, when they're set, which an
// empty list clears; its schema, when it's set, which null clears; and its
// options, each of which is set, or removed, when it's null. What the update
// doesn't set is kept.
func (s GeneratorMeta) Merge(update GeneratorMeta) GeneratorMeta {
	result := s
	if update.Model != nil {
		if model := strings.TrimSpace(*update.Model); model == "" {
			result.Model = nil
		} else {
			result.Model = types.Ptr(model)
		}
	}
	if update.System != nil {
		if system := strings.TrimSpace(*update.System); system == "" {
			result.System = nil
		} else {
			result.System = types.Ptr(*update.System)
		}
	}
	if update.Tools != nil {
		result.Tools = nil
		if len(update.Tools) > 0 {
			result.Tools = slices.Clone(update.Tools)
		}
	}
	if schema := bytes.TrimSpace(update.Schema); len(schema) > 0 {
		if string(schema) == "null" {
			result.Schema = nil
		} else {
			result.Schema = slices.Clone(update.Schema)
		}
	}
	if len(update.Opt) > 0 {
		opt := maps.Clone(s.Opt)
		if opt == nil {
			opt = make(map[string]json.RawMessage, len(update.Opt))
		}
		for key, value := range update.Opt {
			if value := bytes.TrimSpace(value); len(value) == 0 || string(value) == "null" {
				delete(opt, key)
			} else {
				opt[key] = slices.Clone(value)
			}
		}
		if len(opt) == 0 {
			opt = nil
		}
		result.Opt = opt
	}
	return result
}
