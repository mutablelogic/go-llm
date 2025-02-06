package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

// Send a completion request with a single prompt, and return the next completion
func (model *model) Completion(ctx context.Context, prompt string, opts ...llm.Opt) (llm.Completion, error) {
	message, err := messagefactory{}.UserPrompt(prompt, opts...)
	if err != nil {
		return nil, err
	}
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
		Messages:          messages,
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - STREAMING

func streamEvent(response *Response, evt client.TextStreamEvent) error {
	var delta Response
	// If we are done, ignore
	if strings.TrimSpace(evt.Data) == "[DONE]" {
		return nil
	}
	// Decode the event
	if err := evt.Json(&delta); err != nil {
		return err
	}
	// Append the delta to the response
	if delta.Id != "" {
		response.Id = delta.Id
	}
	if delta.Type != "" {
		response.Type = delta.Type
	}
	if delta.Created != 0 {
		response.Created = delta.Created
	}
	if delta.Model != "" {
		response.Model = delta.Model
	}
	if delta.SystemFingerprint != "" {
		response.SystemFingerprint = delta.SystemFingerprint
	}
	if delta.ServiceTier != "" {
		response.ServiceTier = delta.ServiceTier
	}

	// Append the delta to the response
	for _, completion := range delta.Completions {
		if err := appendCompletion(response, &completion); err != nil {
			return err
		}
	}

	// Apend the metrics to the response
	if delta.Metrics != nil {
		response.Metrics = delta.Metrics
	}
	return nil
}

func appendCompletion(response *Response, c *Completion) error {
	fmt.Println(c)
	// Append a new completion
	for {
		if c.Index < uint64(len(response.Completions)) {
			break
		}
		response.Completions = append(response.Completions, Completion{
			Index: c.Index,
			Message: &Message{
				RoleContent: RoleContent{
					Role:    c.Delta.Role(),
					Content: "",
				},
			},
		})
	}

	// Add the reason
	if c.Reason != "" {
		response.Completions[c.Index].Reason = c.Reason
	}

	// Get the completion
	message := response.Completions[c.Index].Message
	if message == nil {
		return llm.ErrBadParameter
	}

	// Add the role
	if role := c.Delta.Role(); role != "" {
		message.RoleContent.Role = role
	}

	// We only allow deltas which are strings at the moment
	if c.Delta.Content != nil {
		if str, ok := c.Delta.Content.(string); ok {
			if text, ok := message.Content.(string); ok {
				message.Content = text + str
			} else {
				message.Content = str
			}
		} else {
			return llm.ErrNotImplemented.Withf("appendCompletion not implemented: %T", c.Delta.Content)
		}
	}

	// Append audio data
	if c.Delta.Media != nil {
		if message.Media == nil {
			message.Media = llm.NewAttachment()
		}
		message.Media.Append(c.Delta.Media)
	}

	// Return success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// COMPLETIONS

// Return the number of completions
func (c Completions) Num() int {
	return len(c)
}

// Return message for a specific completion
func (c Completions) Choice(index int) llm.Completion {
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
