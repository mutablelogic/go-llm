package llm

import (
	"bytes"
	"encoding/json"
	"maps"
	"math"
	"slices"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is an option for an operation of a provider, such as generating a
// message. Options are set by key, so providers can define their own.
type Opt func(*Opts) error

// Opts are the options for an operation, by key
type Opts struct {
	values map[string]any
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Keys for options shared by providers. Each key notes the type of its value.
// Providers ignore or reject the options they don't support, and validate
// ranges, which differ between providers.
const (
	// Sampling
	TemperatureKey      = "temperature"       // float64, randomness of sampling
	TopPKey             = "top_p"             // float64, nucleus sampling probability mass
	TopKKey             = "top_k"             // uint, sample from the K most likely tokens
	MaxTokensKey        = "max_tokens"        // uint, maximum tokens to generate
	StopSequencesKey    = "stop_sequences"    // []string, sequences which stop generation
	SeedKey             = "seed"              // uint, seed for deterministic generation
	PresencePenaltyKey  = "presence_penalty"  // float64, penalty for tokens already present
	FrequencyPenaltyKey = "frequency_penalty" // float64, penalty by frequency of tokens

	// Prompt
	SystemPromptKey = "system_prompt" // string, system prompt for the model

	// Thinking
	ThinkingKey       = "thinking"        // bool, enable or disable thinking
	ThinkingEffortKey = "thinking_effort" // string, reasoning effort: "low", "medium" or "high"

	// Output
	JSONSchemaKey = "json_schema" // json.RawMessage, JSON schema the output conforms to

	// Tools
	ToolChoiceKey = "tool_choice" // string, whether the model uses the tools: "auto", "any" or "none"

	// Model loading
	ContextWindowKey = "context_window" // uint, context window size in tokens
	KeepAliveKey     = "keep_alive"     // time.Duration, how long a model stays loaded, or negative to stay loaded

	// Requests
	UserKey        = "user"         // string, identifier of the end user, for abuse monitoring
	ServiceTierKey = "service_tier" // string, provider service tier, for example "auto"

	// Embeddings
	TaskTypeKey   = "task_type"  // string, task the embedding is used for
	TitleKey      = "title"      // string, title of the document being embedded
	DimensionsKey = "dimensions" // uint, number of dimensions of the embedding
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// ApplyOpts returns the options set by each option in turn, or the first
// error. Nil options are skipped.
func ApplyOpts(opts ...Opt) (*Opts, error) {
	o := &Opts{values: make(map[string]any, len(opts))}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithOpt sets the option with the key to the value, replacing any value
// already set
func WithOpt(key string, value any) Opt {
	return func(o *Opts) error {
		o.values[key] = value
		return nil
	}
}

// WithTemperature sets the randomness of sampling, which is zero or more
func WithTemperature(value float64) Opt {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return withError(ErrBadParameter.Withf("temperature %v must be a number, zero or more", value))
	}
	return WithOpt(TemperatureKey, value)
}

// WithTopP sets the probability mass for nucleus sampling, between zero and one
func WithTopP(value float64) Opt {
	if math.IsNaN(value) || value < 0 || value > 1 {
		return withError(ErrBadParameter.Withf("top_p %v must be between 0 and 1", value))
	}
	return WithOpt(TopPKey, value)
}

// WithTopK sets sampling from the K most likely tokens, which is one or more
func WithTopK(value uint) Opt {
	if value == 0 {
		return withError(ErrBadParameter.With("top_k must be one or more"))
	}
	return WithOpt(TopKKey, value)
}

// WithMaxTokens sets the maximum tokens to generate, which is one or more
func WithMaxTokens(value uint) Opt {
	if value == 0 {
		return withError(ErrBadParameter.With("max_tokens must be one or more"))
	}
	return WithOpt(MaxTokensKey, value)
}

// WithStopSequences sets the sequences which stop generation, replacing any
// already set
func WithStopSequences(values ...string) Opt {
	if slices.Contains(values, "") {
		return withError(ErrBadParameter.With("stop sequences can't be empty"))
	}
	return WithOpt(StopSequencesKey, slices.Clone(values))
}

// WithSeed sets the seed for deterministic generation
func WithSeed(value uint) Opt {
	return WithOpt(SeedKey, value)
}

// WithPresencePenalty sets the penalty for tokens already present
func WithPresencePenalty(value float64) Opt {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return withError(ErrBadParameter.Withf("presence_penalty %v must be a number", value))
	}
	return WithOpt(PresencePenaltyKey, value)
}

// WithFrequencyPenalty sets the penalty by frequency of tokens
func WithFrequencyPenalty(value float64) Opt {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return withError(ErrBadParameter.Withf("frequency_penalty %v must be a number", value))
	}
	return WithOpt(FrequencyPenaltyKey, value)
}

// WithSystemPrompt sets the system prompt for the model
func WithSystemPrompt(value string) Opt {
	return WithOpt(SystemPromptKey, value)
}

// WithThinking enables or disables thinking
func WithThinking(value bool) Opt {
	return WithOpt(ThinkingKey, value)
}

// WithThinkingEffort sets the reasoning effort: "low", "medium" or "high"
func WithThinkingEffort(value string) Opt {
	if !slices.Contains([]string{"low", "medium", "high"}, value) {
		return withError(ErrBadParameter.Withf("thinking_effort %q must be low, medium or high", value))
	}
	return WithOpt(ThinkingEffortKey, value)
}

// WithJSONSchema sets the JSON schema the output conforms to
func WithJSONSchema(value json.RawMessage) Opt {
	if !json.Valid(value) {
		return withError(ErrBadParameter.With("json_schema isn't valid JSON"))
	}
	return WithOpt(JSONSchemaKey, slices.Clone(value))
}

// WithToolChoice sets whether the model uses the tools: "auto" to decide,
// "any" to use one of them, or "none" to use none of them. To use a
// particular tool, pass only that tool with "any".
func WithToolChoice(value string) Opt {
	if !slices.Contains([]string{"auto", "any", "none"}, value) {
		return withError(ErrBadParameter.Withf("tool_choice %q must be auto, any or none", value))
	}
	return WithOpt(ToolChoiceKey, value)
}

// WithContextWindow sets the context window size in tokens, which is one or
// more
func WithContextWindow(value uint) Opt {
	if value == 0 {
		return withError(ErrBadParameter.With("context_window must be one or more"))
	}
	return WithOpt(ContextWindowKey, value)
}

// WithKeepAlive sets how long a model stays loaded after the request, or
// negative to stay loaded
func WithKeepAlive(value time.Duration) Opt {
	return WithOpt(KeepAliveKey, value)
}

// WithUser sets the identifier of the end user, for abuse monitoring
func WithUser(value string) Opt {
	if value == "" {
		return withError(ErrBadParameter.With("user can't be empty"))
	}
	return WithOpt(UserKey, value)
}

// WithServiceTier sets the provider service tier, for example "auto"
func WithServiceTier(value string) Opt {
	if value == "" {
		return withError(ErrBadParameter.With("service_tier can't be empty"))
	}
	return WithOpt(ServiceTierKey, value)
}

// WithTaskType sets the task the embedding is used for
func WithTaskType(value string) Opt {
	if value == "" {
		return withError(ErrBadParameter.With("task_type can't be empty"))
	}
	return WithOpt(TaskTypeKey, value)
}

// WithTitle sets the title of the document being embedded
func WithTitle(value string) Opt {
	return WithOpt(TitleKey, value)
}

// WithDimensions sets the number of dimensions of the embedding, which is one
// or more
func WithDimensions(value uint) Opt {
	if value == 0 {
		return withError(ErrBadParameter.With("dimensions must be one or more"))
	}
	return WithOpt(DimensionsKey, value)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Opt returns an option which sets the values of these options, so options
// can be applied once, and then passed on without being applied again
func (o *Opts) Opt() Opt {
	values := maps.Clone(o.values)
	return func(dest *Opts) error {
		maps.Copy(dest.values, values)
		return nil
	}
}

// Delete removes the option with the key, if it's set
func (o *Opts) Delete(key string) {
	delete(o.values, key)
}

// Has returns true if the option with the key is set
func (o *Opts) Has(key string) bool {
	_, ok := o.values[key]
	return ok
}

// Keys returns the keys of the options which are set, in sorted order
func (o *Opts) Keys() []string {
	return slices.Sorted(maps.Keys(o.values))
}

// Get returns the value of the option with the key, or nil if it isn't set
func (o *Opts) Get(key string) any {
	return o.values[key]
}

// GetOpt returns the value of the option with the key, or an error if it
// isn't set or isn't of the type. A value which is JSON, such as an option
// stored with a session, is decoded as the type, and can't be null.
func GetOpt[T any](o *Opts, key string) (T, error) {
	var value T
	raw, exists := o.values[key]
	if !exists {
		return value, ErrNotFound.Withf("option %q isn't set", key)
	}
	if value, ok := raw.(T); ok {
		return value, nil
	}
	if data, ok := raw.(json.RawMessage); ok {
		if string(bytes.TrimSpace(data)) == "null" {
			return value, ErrBadParameter.Withf("option %q is null, not a %T", key, value)
		} else if err := json.Unmarshal(data, &value); err != nil {
			return value, ErrBadParameter.Withf("option %q isn't a %T: %v", key, value, err)
		}
		return value, nil
	}
	return value, ErrBadParameter.Withf("option %q is a %T, not a %T", key, raw, value)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// withError returns an option which fails with the error, when an option's
// value isn't valid
func withError(err error) Opt {
	return func(*Opts) error {
		return err
	}
}
