package mistral

import (
	"encoding/json"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// GENERATION OPTIONS
//
// See: https://docs.mistral.ai/api/#tag/chat/operation/chat_completion_v1_chat_completions_post

// WithSystemPrompt sets the system prompt for the request.
func WithSystemPrompt(value string) opt.Opt {
	return opt.SetString(opt.SystemPromptKey, value)
}

// WithTemperature sets the temperature for the request (0.0 to 1.5).
// Higher values produce more random output, lower values more deterministic.
func WithTemperature(value float64) opt.Opt {
	if value < 0 || value > 1.5 {
		return opt.Error(llm.ErrBadParameter.With("temperature must be between 0.0 and 1.5"))
	}
	return opt.SetFloat64(opt.TemperatureKey, value)
}

// WithMaxTokens sets the maximum number of tokens to generate (minimum 1).
func WithMaxTokens(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(llm.ErrBadParameter.With("max_tokens must be at least 1"))
	}
	return opt.SetUint(opt.MaxTokensKey, value)
}

// WithTopP sets the nucleus sampling parameter (0.0 to 1.0).
// Tokens are selected from the smallest set whose cumulative probability exceeds top_p.
func WithTopP(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(llm.ErrBadParameter.With("top_p must be between 0.0 and 1.0"))
	}
	return opt.SetFloat64(opt.TopPKey, value)
}

// WithStopSequences sets custom stop sequences for the request.
// Generation stops when any of the specified sequences is encountered.
func WithStopSequences(values ...string) opt.Opt {
	if len(values) == 0 {
		return opt.Error(llm.ErrBadParameter.With("at least one stop sequence is required"))
	}
	return opt.AddString(opt.StopSequencesKey, values...)
}

// WithSeed sets the random seed for deterministic generation.
func WithSeed(value uint) opt.Opt {
	return opt.SetUint(opt.SeedKey, value)
}

// WithPresencePenalty sets the presence penalty (-2.0 to 2.0).
// Positive values penalise tokens that have already appeared, encouraging
// the model to talk about new topics.
func WithPresencePenalty(value float64) opt.Opt {
	if value < -2 || value > 2 {
		return opt.Error(llm.ErrBadParameter.With("presence_penalty must be between -2.0 and 2.0"))
	}
	return opt.SetFloat64(opt.PresencePenaltyKey, value)
}

// WithFrequencyPenalty sets the frequency penalty (-2.0 to 2.0).
// Positive values penalise tokens proportionally to how often they have
// appeared so far, reducing repetition.
func WithFrequencyPenalty(value float64) opt.Opt {
	if value < -2 || value > 2 {
		return opt.Error(llm.ErrBadParameter.With("frequency_penalty must be between -2.0 and 2.0"))
	}
	return opt.SetFloat64(opt.FrequencyPenaltyKey, value)
}

// WithSafePrompt enables the safety prompt injection.
func WithSafePrompt() opt.Opt {
	return opt.SetBool("safe-prompt", true)
}

// WithJSONOutput constrains the model to produce JSON conforming to the given schema.
func WithJSONOutput(schema *jsonschema.Schema) opt.Opt {
	if schema == nil {
		return opt.Error(llm.ErrBadParameter.With("schema is required for JSON output"))
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return opt.Error(llm.ErrBadParameter.Withf("failed to serialize JSON schema: %v", err))
	}
	return opt.SetString(opt.JSONSchemaKey, string(data))
}

///////////////////////////////////////////////////////////////////////////////
// TOOL CHOICE OPTIONS

// WithToolChoiceAuto lets the model decide whether to use tools.
func WithToolChoiceAuto() opt.Opt {
	return opt.SetString(opt.ToolChoiceKey, toolChoiceAuto)
}

// WithToolChoiceNone prevents the model from using any tools.
func WithToolChoiceNone() opt.Opt {
	return opt.SetString(opt.ToolChoiceKey, toolChoiceNone)
}

// WithToolChoiceAny forces the model to use one of the available tools.
func WithToolChoiceAny() opt.Opt {
	return opt.SetString(opt.ToolChoiceKey, toolChoiceAny)
}

// WithToolChoiceRequired forces the model to use a tool (alias for "required").
func WithToolChoiceRequired() opt.Opt {
	return opt.SetString(opt.ToolChoiceKey, toolChoiceRequired)
}
