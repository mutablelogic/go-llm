package openai

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	session "github.com/mutablelogic/go-llm/pkg/session"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Completion Response
type Response struct {
	Type              string `json:"object"`
	Created           uint64 `json:"created"`
	Model             string `json:"model"`
	SystemFingerprint string `json:"system_fingerprint"`
	ServiceTier       string `json:"service_tier"`
	Completions       `json:"choices"`
	Metrics           `json:"usage,omitempty"`
}

// Metrics
type Metrics struct {
	PromptTokens           uint64 `json:"prompt_tokens,omitempty"`
	CompletionTokens       uint64 `json:"completion_tokens,omitempty"`
	TotalTokens            uint64 `json:"total_tokens,omitempty"`
	CompletionTokenDetails struct {
		ReasoningTokens          uint64 `json:"reasoning_tokens,omitempty"`
		AcceptedPredictionTokens uint64 `json:"accepted_prediction_tokens,omitempty"`
		RejectedPredictionTokens uint64 `json:"rejected_prediction_tokens,omitempty"`
	} `json:"completion_token_details,omitempty"`
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
	LogProbs          *bool             `json:"logprobs,omitempty"`
	TopLogProbs       uint64            `json:"top_logprobs,omitempty"`
	MaxTokens         uint64            `json:"max_completion_tokens,omitempty"`
	NumChoices        uint64            `json:"n,omitempty"`
	Modalties         []string          `json:"modalities,omitempty"`
	Prediction        *Content          `json:"prediction,omitempty"`
	Audio             *Audio            `json:"audio,omitempty"`
	PresencePenalty   float64           `json:"presence_penalty,omitempty"`
	Format            *Format           `json:"response_format,omitempty"`
	Seed              uint64            `json:"random_seed,omitempty"`
	ServiceTier       string            `json:"service_tier,omitempty"`
	StopSequences     []string          `json:"stop,omitempty"`
	Stream            *bool             `json:"stream,omitempty"`
	StreamOptions     *StreamOptions    `json:"stream_options,omitempty"`
	Temperature       float64           `json:"temperature,omitempty"`
	TopP              float64           `json:"top_p,omitempty"`
	Tools             []llm.Tool        `json:"tools,omitempty"`
	ToolChoice        any               `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool             `json:"parallel_tool_calls,omitempty"`
	User              string            `json:"user,omitempty"`
	Messages          []llm.Completion  `json:"messages"`
}

func (model *model) Completion(ctx context.Context, session llm.Context, opts ...llm.Opt) (*Response, error) {
	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Request
	req, err := client.NewJSONRequest(reqCompletion{
		Model:            model.Name(),
		Temperature:      optTemperature(opt),
		TopP:             optTopP(opt),
		MaxTokens:        optMaxTokens(opt),
		Stream:           optStream(opt),
		StopSequences:    optStopSequences(opt),
		Seed:             optSeed(opt),
		Messages:         messages,
		Format:           optFormat(opt),
		Tools:            optTools(mistral, opt),
		ToolChoice:       optToolChoice(opt),
		PresencePenalty:  optPresencePenalty(opt),
		FrequencyPenalty: optFrequencyPenalty(opt),
		NumChoices:       optNumCompletions(opt),
		Prediction:       optPrediction(opt),
		SafePrompt:       optSafePrompt(opt),
	})
	if err != nil {
		return nil, err
	}

	var response Response
	reqopts := []client.RequestOpt{
		client.OptPath("chat", "completions"),
	}
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
	if err := mistral.DoWithContext(ctx, req, &response, reqopts...); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

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
	if delta.Created != 0 {
		response.Created = delta.Created
	}
	if delta.Model != "" {
		response.Model = delta.Model
	}
	for _, completion := range delta.Completions {
		appendCompletion(response, &completion)
	}
	if delta.Metrics.InputTokens > 0 {
		response.Metrics.InputTokens += delta.Metrics.InputTokens
	}
	if delta.Metrics.OutputTokens > 0 {
		response.Metrics.OutputTokens += delta.Metrics.OutputTokens
	}
	if delta.Metrics.TotalTokens > 0 {
		response.Metrics.TotalTokens += delta.Metrics.TotalTokens
	}
	return nil
}

func appendCompletion(response *Response, c *Completion) {
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
	// Add the completion delta
	if c.Reason != "" {
		response.Completions[c.Index].Reason = c.Reason
	}
	if role := c.Delta.Role(); role != "" {
		response.Completions[c.Index].Message.RoleContent.Role = role
	}

	// TODO: We only allow deltas which are strings at the moment...
	if str, ok := c.Delta.Content.(string); ok && str != "" {
		if text, ok := response.Completions[c.Index].Message.Content.(string); ok {
			response.Completions[c.Index].Message.Content = text + str
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Generate a completion from a prompt without any history
func (model *model) Completion(prompt string, opts ...llm.Opt) (llm.Completion, error) {
	// Create a new session
	session := session.NewSession(model, &messagefactory{}, opts...)

	// Append a user prompt
	message, err := messagefactory{}.UserPrompt(prompt, opts...)
	if err != nil {
		panic(err)
	}
	session.Append(message)

	return session
}
