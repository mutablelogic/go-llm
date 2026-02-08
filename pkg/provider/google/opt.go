package google

import (
	"encoding/json"
	"fmt"

	// Packages
	"github.com/google/jsonschema-go/jsonschema"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// GENERATION OPTIONS
//
// See: https://ai.google.dev/gemini-api/docs/text-generation

// WithSystemPrompt sets the system instruction for the request.
//
// See: https://ai.google.dev/gemini-api/docs/system-instructions
func WithSystemPrompt(value string) opt.Opt {
	return opt.SetString(opt.SystemPromptKey, value)
}

// WithTemperature sets the temperature for the request (0.0 to 2.0).
// Higher values produce more random output, lower values more deterministic.
//
// See: https://ai.google.dev/gemini-api/docs/text-generation#configuration-parameters
func WithTemperature(value float64) opt.Opt {
	if value < 0 || value > 2 {
		return opt.Error(fmt.Errorf("temperature must be between 0.0 and 2.0"))
	}
	return opt.SetFloat64(opt.TemperatureKey, value)
}

// WithMaxTokens sets the maximum number of tokens to generate (minimum 1).
//
// See: https://ai.google.dev/gemini-api/docs/text-generation#configuration-parameters
func WithMaxTokens(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("max_tokens must be at least 1"))
	}
	return opt.SetUint(opt.MaxTokensKey, value)
}

// WithTopK sets the top-K sampling parameter (minimum 1).
// Limits token selection to the K most probable tokens.
//
// See: https://ai.google.dev/gemini-api/docs/text-generation#configuration-parameters
func WithTopK(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("top_k must be at least 1"))
	}
	return opt.SetUint(opt.TopKKey, value)
}

// WithTopP sets the nucleus sampling parameter (0.0 to 1.0).
// Tokens are selected from the smallest set whose cumulative probability exceeds top_p.
//
// See: https://ai.google.dev/gemini-api/docs/text-generation#configuration-parameters
func WithTopP(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(fmt.Errorf("top_p must be between 0.0 and 1.0"))
	}
	return opt.SetFloat64(opt.TopPKey, value)
}

// WithStopSequences sets custom stop sequences for the request.
// Generation stops when any of the specified sequences is encountered.
//
// See: https://ai.google.dev/gemini-api/docs/text-generation#configuration-parameters
func WithStopSequences(values ...string) opt.Opt {
	if len(values) == 0 {
		return opt.Error(fmt.Errorf("at least one stop sequence is required"))
	}
	return opt.AddString(opt.StopSequencesKey, values...)
}

// WithThinking enables the model's extended thinking/reasoning capability.
// When enabled, the model may include internal reasoning in its response.
// Only supported by models that have thinking capabilities (e.g. gemini-2.5-flash).
//
// See: https://ai.google.dev/gemini-api/docs/thinking
func WithThinking() opt.Opt {
	return opt.SetBool(opt.ThinkingKey, true)
}

// WithJSONOutput constrains the model to produce JSON conforming to the given schema.
// Sets responseMimeType to "application/json" and responseJsonSchema on the request.
//
// See: https://ai.google.dev/gemini-api/docs/json-mode
func WithJSONOutput(schema *jsonschema.Schema) opt.Opt {
	if schema == nil {
		return opt.Error(fmt.Errorf("schema is required for JSON output"))
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return opt.Error(fmt.Errorf("failed to serialize JSON schema: %w", err))
	}
	return opt.SetString(opt.JSONSchemaKey, string(data))
}

// WithSeed sets the seed for deterministic generation.
// Using the same seed with the same inputs should produce repeatable results.
//
// See: https://ai.google.dev/gemini-api/docs/text-generation#configuration-parameters
func WithSeed(value int) opt.Opt {
	return opt.SetAny(opt.SeedKey, value)
}

// WithPresencePenalty sets the presence penalty (-2.0 to 2.0).
// Positive values penalise tokens that have already appeared, encouraging
// the model to talk about new topics.
//
// See: https://ai.google.dev/gemini-api/docs/text-generation#configuration-parameters
func WithPresencePenalty(value float64) opt.Opt {
	if value < -2 || value > 2 {
		return opt.Error(fmt.Errorf("presence_penalty must be between -2.0 and 2.0"))
	}
	return opt.SetFloat64(opt.PresencePenaltyKey, value)
}

// WithFrequencyPenalty sets the frequency penalty (-2.0 to 2.0).
// Positive values penalise tokens proportionally to how often they have
// appeared so far, reducing repetition.
//
// See: https://ai.google.dev/gemini-api/docs/text-generation#configuration-parameters
func WithFrequencyPenalty(value float64) opt.Opt {
	if value < -2 || value > 2 {
		return opt.Error(fmt.Errorf("frequency_penalty must be between -2.0 and 2.0"))
	}
	return opt.SetFloat64(opt.FrequencyPenaltyKey, value)
}

///////////////////////////////////////////////////////////////////////////////
// EMBEDDING OPTIONS
//
// See: https://ai.google.dev/gemini-api/docs/embeddings

// WithTaskType sets the task type for the embedding request.
// Supported values: SEMANTIC_SIMILARITY, CLASSIFICATION, CLUSTERING,
// RETRIEVAL_DOCUMENT, RETRIEVAL_QUERY, CODE_RETRIEVAL_QUERY,
// QUESTION_ANSWERING, FACT_VERIFICATION.
//
// See: https://ai.google.dev/gemini-api/docs/embeddings#specify-task-type
func WithTaskType(taskType string) opt.Opt {
	return opt.SetString(opt.TaskTypeKey, taskType)
}

// WithTitle sets the title for the embedding request.
// Only applicable when TaskType is RETRIEVAL_DOCUMENT.
//
// See: https://ai.google.dev/gemini-api/docs/embeddings#specify-task-type
func WithTitle(title string) opt.Opt {
	return opt.SetString(opt.TitleKey, title)
}

// WithOutputDimensionality sets the output dimensionality for the embedding.
// The default is 3072. Recommended values are 768, 1536, or 3072.
// Smaller dimensions save storage space with minimal quality loss.
//
// See: https://ai.google.dev/gemini-api/docs/embeddings#controlling-embedding-size
func WithOutputDimensionality(d uint) opt.Opt {
	return opt.SetUint(opt.OutputDimensionalityKey, d)
}
