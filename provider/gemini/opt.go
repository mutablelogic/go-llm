package gemini

import (
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// thinkingBudgets are the budgets of tokens Gemini 2.5 thinks within, for
// each effort, where -1 is as much as it decides
var thinkingBudgets = map[string]int{"low": 1024, "medium": 8192, "high": -1}

// minThinkingBudget is the least Gemini 2.5 Pro, which can't stop thinking,
// thinks within
const minThinkingBudget = 128

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// apply sets the options on the request, for the model, or returns an error
// if an option isn't valid. The user, the service tier, and the context
// window, which the API hasn't, are ignored, as a conversation keeps its
// options as its model changes.
func (r *generateRequest) apply(model *Model, opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	config := &generationConfig{}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.TemperatureKey:
			var value float64
			if value, err = llm.GetOpt[float64](o, key); err == nil {
				config.Temperature = types.Ptr(value)
			}
		case llm.TopPKey:
			var value float64
			if value, err = llm.GetOpt[float64](o, key); err == nil {
				config.TopP = types.Ptr(value)
			}
		case llm.TopKKey:
			var value uint
			if value, err = llm.GetOpt[uint](o, key); err == nil {
				config.TopK = types.Ptr(value)
			}
		case llm.MaxTokensKey:
			config.MaxOutputTokens, err = llm.GetOpt[uint](o, key)
		case llm.StopSequencesKey:
			config.StopSequences, err = llm.GetOpt[[]string](o, key)
		case llm.SeedKey:
			var value uint
			if value, err = llm.GetOpt[uint](o, key); err == nil {
				config.Seed = types.Ptr(value)
			}
		case llm.PresencePenaltyKey:
			var value float64
			if value, err = llm.GetOpt[float64](o, key); err == nil {
				config.PresencePenalty = types.Ptr(value)
			}
		case llm.FrequencyPenaltyKey:
			var value float64
			if value, err = llm.GetOpt[float64](o, key); err == nil {
				config.FrequencyPenalty = types.Ptr(value)
			}
		case llm.SystemPromptKey:
			var prompt string
			if prompt, err = llm.GetOpt[string](o, key); err == nil && strings.TrimSpace(prompt) != "" {
				r.system(prompt)
			}
		case llm.ThinkingKey:
			_, err = llm.GetOpt[bool](o, key)
		case llm.ThinkingEffortKey:
			var effort string
			if effort, err = llm.GetOpt[string](o, key); err == nil {
				if _, ok := thinkingBudgets[effort]; !ok {
					err = llm.ErrBadParameter.Withf("thinking effort %q isn't low, medium or high", effort)
				}
			}
		case llm.JSONSchemaKey:
			var schema json.RawMessage
			if schema, err = llm.GetOpt[json.RawMessage](o, key); err == nil {
				if !json.Valid(schema) {
					err = llm.ErrBadParameter.With("invalid json_schema")
				} else {
					config.ResponseMimeType, config.ResponseJSONSchema = "application/json", schema
				}
			}
		case llm.ToolChoiceKey:
			var choice string
			if choice, err = llm.GetOpt[string](o, key); err == nil {
				switch choice {
				case "auto", "any", "none":
					r.ToolConfig = &toolConfig{FunctionCallingConfig: functionCallingConfig{Mode: strings.ToUpper(choice)}}
				default:
					err = llm.ErrBadParameter.Withf("tool choice %q isn't supported", choice)
				}
			}
		case llm.UserKey, llm.ServiceTierKey, llm.ContextWindowKey:
			// Ignored, as the API hasn't them
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by gemini", key)
		}
		if err != nil {
			return err
		}
	}

	// How the model thinks
	think, thinkErr := llm.GetOpt[bool](o, llm.ThinkingKey)
	effort, _ := llm.GetOpt[string](o, llm.ThinkingEffortKey)
	thinking, err := model.thinking(thinkErr == nil, think, effort)
	if err != nil {
		return err
	}
	config.ThinkingConfig = thinking

	// The configuration, unless it's empty, and a tool choice, which needs
	// tools
	if data, err := json.Marshal(config); err != nil {
		return llm.ErrBadParameter.Withf("invalid options: %v", err)
	} else if string(data) != "{}" {
		r.GenerationConfig = config
	}
	if len(r.Tools) == 0 {
		r.ToolConfig = nil
	}
	return nil
}

// system adds the prompt before the system instruction
func (r *generateRequest) system(prompt string) {
	text := strings.TrimSpace(prompt)
	if r.SystemInstruction != nil && len(r.SystemInstruction.Parts) > 0 {
		text = strings.TrimSpace(text + "\n\n" + r.SystemInstruction.Parts[0].Text)
	}
	r.SystemInstruction = &content{Parts: []part{{Text: text}}}
}

// thinking returns how the model thinks, as it's asked: its thoughts are
// summarized, so they're shown, unless thinking is off. Gemini 2.5 thinks
// within a budget, for the effort, or not at all, when thinking is off,
// apart from Gemini 2.5 Pro, which thinks the least it can, and later
// models think at a level, of the effort, or the lowest, when thinking is
// off, as they can't stop thinking. A model which doesn't think, or isn't
// known to, isn't told how to, and can't be asked to.
func (m *Model) thinking(asked, think bool, effort string) (*thinkingConfig, error) {
	if !types.Value(m.data.Thinking) {
		// Thinking, or an effort, which asks it to think, unless thinking is
		// off, can't be asked of a model which doesn't think
		if (asked && think) || (effort != "" && !(asked && !think)) {
			return nil, llm.ErrBadParameter.Withf("%s doesn't think", m.Name())
		}
		return nil, nil
	}
	off := asked && !think
	config := &thinkingConfig{IncludeThoughts: !off}
	if strings.HasPrefix(m.Name(), "gemini-2") {
		switch {
		case off && strings.Contains(m.Name(), "-pro"):
			config.ThinkingBudget = types.Ptr(minThinkingBudget)
		case off:
			config.ThinkingBudget = types.Ptr(0)
		case effort != "":
			config.ThinkingBudget = types.Ptr(thinkingBudgets[effort])
		}
		return config, nil
	}
	switch {
	case off:
		config.ThinkingLevel = "LOW"
	case effort != "":
		config.ThinkingLevel = strings.ToUpper(effort)
	}
	return config, nil
}
