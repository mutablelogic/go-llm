package openai

import (
	// Packages
	"slices"
	"strings"

	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embeddings: The number of dimensions the resulting output embeddings
// should have. Only supported in text-embedding-3 and later models.
func WithDimensions(v uint64) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("dimensions", v)
		return nil
	}
}

// Whether or not to store the output of this chat completion request for use in
// model distillation or evals products.
func WithStore(v bool) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("store", v)
		return nil
	}
}

// Constrains effort on reasoning for reasoning models. Currently supported values are
// low, medium, and high. Reducing reasoning effort can result in faster responses
// and fewer tokens used on reasoning in a response.
func WithReasoningEffort(v string) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("reasoning_effort", v)
		return nil
	}
}

// Key-value pair that can be attached to an object. This can be useful for storing
// additional information about the object in a structured format, and querying for objects
// via API or the dashboard.
func WithMetadata(k, v string) llm.Opt {
	return func(o *llm.Opts) error {
		// Set store to true
		if err := WithStore(true)(o); err != nil {
			return err
		}

		// Add metadata
		metadata, ok := o.Get("metadata").(map[string]string)
		if !ok {
			metadata = make(map[string]string, 16)
		}
		metadata[k] = v
		o.Set("metadata", metadata)
		return nil
	}
}

// Tokens (specified by their token ID in the tokenizer) to an associated bias
// value from -100 to 100. Mathematically, the bias is added to the logits
// generated by the model prior to sampling. The exact effect will vary per model,
// but values between -1 and 1 should decrease or increase likelihood of selection;
// values like -100 or 100 should result in a ban or exclusive selection of the
// relevant token.
func WithLogitBias(token uint64, bias int64) llm.Opt {
	return func(o *llm.Opts) error {
		logit_bias, ok := o.Get("logit_bias").(map[uint64]int64)
		if !ok {
			logit_bias = make(map[uint64]int64, 16)
		}
		logit_bias[token] = bias
		o.Set("logit_bias", logit_bias)
		return nil
	}
}

// Whether to return log probabilities of the output tokens or not.
func WithLogProbs() llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("logprobs", true)
		return nil
	}
}

// An integer between 0 and 20 specifying the number of most likely tokens
// to return at each token position, each with an associated log probability.
func WithTopLogProbs(v uint64) llm.Opt {
	return func(o *llm.Opts) error {
		if v > 20 {
			return llm.ErrBadParameter.With("top_logprobs")
		}
		o.Set("logprobs", true)
		o.Set("top_logprobs", v)
		return nil
	}
}

// Output types that you would like the model to generate for this request.
// Supported values are: "text", "audio"
func WithModalities(v ...string) llm.Opt {
	return func(o *llm.Opts) error {
		arr, ok := o.Get("modalities").([]string)
		if !ok {
			arr = make([]string, 0, 16)
		}
		for _, v := range v {
			v = strings.ToLower(strings.TrimSpace(v))
			if !slices.Contains(arr, v) {
				arr = append(arr, v)
			}
		}
		o.Set("modalities", arr)
		return nil
	}
}

// Parameters for audio output
func WithAudio(voice, format string) llm.Opt {
	return func(o *llm.Opts) error {
		if err := WithModalities("text", "audio")(o); err != nil {
			return err
		}
		if audio := NewAudio(voice, format); audio != nil {
			o.Set("audio", audio)
		} else {
			return llm.ErrBadParameter.With("audio")
		}
		return nil
	}
}

// Parameters for speech output
func WithAudioSpeed(v float64) llm.Opt {
	return func(o *llm.Opts) error {
		if v < 0.25 || v > 4.0 {
			return llm.ErrBadParameter.With("speed")
		}
		o.Set("speed", v)
		return nil
	}
}

// Parameters for image output
func WithSize(v string) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("size", v)
		return nil
	}
}

// Parameters for image output
func WithQuality(v string) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("quality", v)
		return nil
	}
}

// Parameters for image output
func WithStyle(v string) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("style", v)
		return nil
	}
}

// Specifies the latency tier to use for processing the request. Values
// can be auto or default
func WithServiceTier(v string) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("service_tier", v)
		return nil
	}
}

// Enable streaming and include usage information in the streaming response
func WithStreamOptions(fn func(llm.Completion), include_usage bool) llm.Opt {
	return func(o *llm.Opts) error {
		if err := llm.WithStream(fn)(o); err != nil {
			return err
		}
		o.Set("stream_options_include_usage", include_usage)
		return nil
	}
}

// Disable parallel tool calling
func WithDisableParallelToolCalls() llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("parallel_tool_calls", false)
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// For embedding
func optFormat(opts *llm.Opts) string {
	return opts.GetString("format")
}

// For embedding
func optDimensions(opts *llm.Opts) uint64 {
	return opts.GetUint64("dimensions")
}

// For embedding and completions
func optUser(opts *llm.Opts) string {
	return opts.GetString("user")
}

func optStore(opts *llm.Opts) *bool {
	if v, ok := opts.Get("store").(bool); ok {
		return &v
	}
	return nil
}

func optReasoningEffort(opts *llm.Opts) string {
	return opts.GetString("reasoning_effort")
}

func optMetadata(opts *llm.Opts) map[string]string {
	if metadata, ok := opts.Get("metadata").(map[string]string); ok {
		return metadata
	}
	return nil
}

func optFrequencyPenalty(opts *llm.Opts) float64 {
	return opts.GetFloat64("frequency_penalty")
}

func optLogitBias(opts *llm.Opts) map[uint64]int64 {
	if logit_bias, ok := opts.Get("logit_bias").(map[uint64]int64); ok {
		return logit_bias
	}
	return nil
}

func optLogProbs(opts *llm.Opts) bool {
	return opts.GetBool("logprobs")
}

func optTopLogProbs(opts *llm.Opts) uint64 {
	return opts.GetUint64("top_logprobs")
}

func optMaxTokens(opts *llm.Opts) uint64 {
	return opts.GetUint64("max_tokens")
}

func optNumCompletions(opts *llm.Opts) uint64 {
	return opts.GetUint64("num_completions")
}

func optModalities(opts *llm.Opts) []string {
	if v, ok := opts.Get("modalities").([]string); ok {
		return v
	}
	return nil
}

func optPrediction(opts *llm.Opts) *Content {
	v := strings.TrimSpace(opts.GetString("prediction"))
	if v != "" {
		return NewContentString("content", v)
	}
	return nil
}

func optAudio(opts *llm.Opts) *Audio {
	v, ok := opts.Get("audio").(*Audio)
	if ok {
		return v
	}
	if v == nil {
		opts.Set("audio", NewAudio("ash", "mp3"))
		return optAudio(opts)
	}
	return nil
}

func optPresencePenalty(opts *llm.Opts) float64 {
	return opts.GetFloat64("presence_penalty")
}

func optResponseFormat(opts *llm.Opts) *Format {
	if format := NewFormat(optFormat(opts)); format != nil {
		return format
	} else {
		return nil
	}
}

func optSeed(opts *llm.Opts) uint64 {
	return opts.GetUint64("seed")
}

func optServiceTier(opts *llm.Opts) string {
	return opts.GetString("service_tier")
}

func optStreamOptions(opts *llm.Opts) *StreamOptions {
	if opts.Has("stream_options_include_usage") {
		return NewStreamOptions(opts.GetBool("stream_options_include_usage"))
	} else {
		return nil
	}
}

func optStream(opts *llm.Opts) bool {
	return opts.StreamFn() != nil
}

func optTemperature(opts *llm.Opts) float64 {
	return opts.GetFloat64("temperature")
}

func optTopP(opts *llm.Opts) float64 {
	return opts.GetFloat64("top_p")
}

func optStopSequences(opts *llm.Opts) []string {
	if opts.Has("stop") {
		if stop, ok := opts.Get("stop").([]string); ok {
			return stop
		}
	}
	return nil
}

func optTools(agent llm.Agent, opts *llm.Opts) []llm.Tool {
	toolkit := opts.ToolKit()
	if toolkit == nil {
		return nil
	}
	return toolkit.Tools(agent)
}

func optToolChoice(opts *llm.Opts) any {
	choices, ok := opts.Get("tool_choice").([]string)
	if !ok || len(choices) == 0 {
		return nil
	}

	// We only support one choice
	choice := strings.TrimSpace(strings.ToLower(choices[0]))
	switch choice {
	case "auto", "none", "required":
		return choice
	case "":
		return nil
	default:
		return NewToolChoice(choice)
	}
}

func optParallelToolCalls(opts *llm.Opts) *bool {
	if opts.Has("parallel_tool_calls") {
		v := opts.GetBool("parallel_tool_calls")
		return &v
	}
	return nil
}
