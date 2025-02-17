package deepseek

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
	*Metrics          `json:"usage,omitempty"`
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

func (c Completion) String() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (m Metrics) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

type reqCompletion struct {
	Model            string           `json:"model"`
	FrequencyPenalty float64          `json:"frequency_penalty,omitempty"`
	MaxTokens        uint64           `json:"max_tokens,omitempty"`
	PresencePenalty  float64          `json:"presence_penalty,omitempty"`
	ResponseFormat   *Format          `json:"response_format,omitempty"`
	StopSequences    []string         `json:"stop,omitempty"`
	Stream           bool             `json:"stream,omitempty"`
	Temperature      float64          `json:"temperature,omitempty"`
	TopP             float64          `json:"top_p,omitempty"`
	Tools            []llm.Tool       `json:"tools,omitempty"`
	ToolChoice       any              `json:"tool_choice,omitempty"`
	LogProbs         bool             `json:"logprobs,omitempty"`
	TopLogProbs      uint64           `json:"top_logprobs,omitempty"`
	Messages         []llm.Completion `json:"messages"`
}

// Send a completion request with a single prompt, and return the next completion
func (model *model) Completion(ctx context.Context, prompt string, opts ...llm.Opt) (llm.Completion, error) {
	// Create a user prompt
	message, err := messagefactory{}.UserPrompt(prompt, opts...)
	if err != nil {
		return nil, err
	}

	// Chat completion
	return model.Chat(ctx, []llm.Completion{message}, opts...)
}

// Send a completion request with multiple completions, and return the next completion
func (model *model) Chat(ctx context.Context, completions []llm.Completion, opts ...llm.Opt) (llm.Completion, error) {
	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Create the completions including the system prompt
	messages := make([]llm.Completion, 0, len(completions)+1)
	if system := opt.SystemPrompt(); system != "" {
		messages = append(messages, messagefactory{}.SystemPrompt(system))
	}
	messages = append(messages, completions...)

	// Request
	req, err := client.NewJSONRequest(reqCompletion{
		Model:            model.Name(),
		FrequencyPenalty: optFrequencyPenalty(opt),
		MaxTokens:        optMaxTokens(opt),
		PresencePenalty:  optPresencePenalty(opt),
		ResponseFormat:   optResponseFormat(opt),
		StopSequences:    optStopSequences(opt),
		Stream:           optStream(opt),
		Temperature:      optTemperature(opt),
		TopP:             optTopP(opt),
		Tools:            optTools(model, opt),
		ToolChoice:       optToolChoice(opt),
		LogProbs:         optLogProbs(opt),
		TopLogProbs:      optTopLogProbs(opt),
		Messages:         messages,
	})
	if err != nil {
		return nil, err
	}

	// Response options
	var response Response
	reqopts := []client.RequestOpt{
		client.OptPath("chat", "completions"),
	}

	// Streaming
	if optStream(opt) {
		reqopts = append(reqopts, client.OptTextStreamCallback(func(evt client.TextStreamEvent) error {
			if err := streamEvent(&response, evt); err != nil {
				return err
			}
			if fn := opt.StreamFn(); fn != nil {
				fn(&response)
			}
			return nil
		}))
	}

	// Response
	if err := model.DoWithContext(ctx, req, &response, reqopts...); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}
