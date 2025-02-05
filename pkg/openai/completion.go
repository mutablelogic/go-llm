package openai

import (
	"context"
	"encoding/json"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Completion Response
type Response struct {
	Id                string `json:"id"`
	Type              string `json:"object"`
	Created           uint64 `json:"created"`
	Model             string `json:"model"`
	SystemFingerprint string `json:"system_fingerprint"`
	ServiceTier       string `json:"service_tier"`
	Completions       `json:"choices"`
	Metrics           `json:"usage,omitempty"`
}

// Completion choices
type Completions []Completion

// Completion Variation
type Completion struct {
	Index   uint64   `json:"index"`
	Message *Message `json:"message"`
	Delta   *Message `json:"delta,omitempty"` // For streaming
	Reason  string   `json:"finish_reason,omitempty"`
}

// Metrics
type Metrics struct {
	PromptTokens       uint64 `json:"prompt_tokens,omitempty"`
	CompletionTokens   uint64 `json:"completion_tokens,omitempty"`
	TotalTokens        uint64 `json:"total_tokens,omitempty"`
	PromptTokenDetails struct {
		CachedTokens uint64 `json:"cached_tokens,omitempty"`
		AudioTokens  uint64 `json:"audio_tokens,omitempty"`
	} `json:"prompt_tokens_details,omitempty"`
	CompletionTokenDetails struct {
		ReasoningTokens          uint64 `json:"reasoning_tokens,omitempty"`
		AcceptedPredictionTokens uint64 `json:"accepted_prediction_tokens,omitempty"`
		RejectedPredictionTokens uint64 `json:"rejected_prediction_tokens,omitempty"`
	} `json:"completion_tokens_details,omitempty"`
}

var _ llm.Completion = (*Response)(nil)

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r Response) String() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

type reqCompletion struct {
	Model             string            `json:"model"`
	Store             *bool             `json:"store,omitempty"`
	ReasoningEffort   string            `json:"reasoning_effort,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	FrequencyPenalty  float64           `json:"frequency_penalty,omitempty"`
	LogitBias         map[uint64]int64  `json:"logit_bias,omitempty"`
	LogProbs          bool              `json:"logprobs,omitempty"`
	TopLogProbs       uint64            `json:"top_logprobs,omitempty"`
	MaxTokens         uint64            `json:"max_completion_tokens,omitempty"`
	NumCompletions    uint64            `json:"n,omitempty"`
	Modalties         []string          `json:"modalities,omitempty"`
	Prediction        *Content          `json:"prediction,omitempty"`
	Audio             *Audio            `json:"audio,omitempty"`
	PresencePenalty   float64           `json:"presence_penalty,omitempty"`
	ResponseFormat    *Format           `json:"response_format,omitempty"`
	Seed              uint64            `json:"seed,omitempty"`
	ServiceTier       string            `json:"service_tier,omitempty"`
	StopSequences     []string          `json:"stop,omitempty"`
	Stream            bool              `json:"stream,omitempty"`
	StreamOptions     *StreamOptions    `json:"stream_options,omitempty"`
	Temperature       float64           `json:"temperature,omitempty"`
	TopP              float64           `json:"top_p,omitempty"`
	Tools             []llm.Tool        `json:"tools,omitempty"`
	ToolChoice        any               `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool             `json:"parallel_tool_calls,omitempty"`
	User              string            `json:"user,omitempty"`
	Messages          []llm.Completion  `json:"messages"`
}

func (model *model) Completion(ctx context.Context, prompt string, opts ...llm.Opt) (llm.Completion, error) {
	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// TODO: Add a system message

	// Create a message
	message, err := messagefactory{}.UserPrompt(prompt, opts...)
	if err != nil {
		return nil, err
	}

	// Request
	req, err := client.NewJSONRequest(reqCompletion{
		Model:             model.Name(),
		Store:             optStore(opt),
		ReasoningEffort:   optReasoningEffort(opt),
		Metadata:          optMetadata(opt),
		FrequencyPenalty:  optFrequencyPenalty(opt),
		LogitBias:         optLogitBias(opt),
		LogProbs:          optLogProbs(opt),
		TopLogProbs:       optTopLogProbs(opt),
		MaxTokens:         optMaxTokens(opt),
		NumCompletions:    optNumCompletions(opt),
		Modalties:         optModalities(opt),
		Prediction:        optPrediction(opt),
		Audio:             optAudio(opt),
		PresencePenalty:   optPresencePenalty(opt),
		ResponseFormat:    optResponseFormat(opt),
		Seed:              optSeed(opt),
		ServiceTier:       optServiceTier(opt),
		StreamOptions:     optStreamOptions(opt),
		Temperature:       optTemperature(opt),
		TopP:              optTopP(opt),
		Stream:            optStream(opt),
		StopSequences:     optStopSequences(opt),
		Tools:             optTools(model, opt),
		ToolChoice:        optToolChoice(opt),
		ParallelToolCalls: optParallelToolCalls(opt),
		User:              optUser(opt),
		Messages:          []llm.Completion{message},
	})
	if err != nil {
		return nil, err
	}

	var response Response
	reqopts := []client.RequestOpt{
		client.OptPath("chat", "completions"),
	}

	// Response
	if err := model.DoWithContext(ctx, req, &response, reqopts...); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// COMPLETIONS

// Return the number of completions
func (c Completions) Num() int {
	return len(c)
}

// Return message for a specific completion
func (c Completions) Message(index int) *Message {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message
}

// Return the role of the completion
func (c Completions) Role() string {
	// The role should be the same for all completions, let's use the first one
	if len(c) == 0 {
		return ""
	}
	return c[0].Message.Role()
}

// Return the text content for a specific completion
func (c Completions) Text(index int) string {
	if index < 0 || index >= len(c) {
		return ""
	}
	return c[index].Message.Text(0)
}

// Return audio content for a specific completion
func (c Completions) Audio(index int) *llm.Attachment {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message.Audio(0)
}

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (c Completions) ToolCalls(index int) []llm.ToolCall {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message.ToolCalls(0)
}
