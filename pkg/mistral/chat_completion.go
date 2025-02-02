package mistral

import (
	"context"
	"encoding/json"

	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Chat Completion Response
type Response struct {
	Id      string   `json:"id"`
	Type    string   `json:"object"`
	Created uint64   `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Metrics `json:"usage,omitempty"`
}

// Response variation
type Choice struct {
	Index   uint64      `json:"index"`
	Message MessageMeta `json:"message"`
	Reason  string      `json:"finish_reason,omitempty"`
}

// Metrics
type Metrics struct {
	InputTokens  uint64 `json:"prompt_tokens,omitempty"`
	OutputTokens uint   `json:"completion_tokens,omitempty"`
	TotalTokens  uint   `json:"total_tokens,omitempty"`
}

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

type reqChatCompletion struct {
	Model            string         `json:"model"`
	Temperature      float64        `json:"temperature,omitempty"`
	TopP             float64        `json:"top_p,omitempty"`
	MaxTokens        uint64         `json:"max_tokens,omitempty"`
	Stream           bool           `json:"stream,omitempty"`
	StopSequences    []string       `json:"stop,omitempty"`
	Seed             uint64         `json:"random_seed,omitempty"`
	Messages         []*MessageMeta `json:"messages"`
	Format           any            `json:"response_format,omitempty"`
	Tools            []llm.Tool     `json:"tools,omitempty"`
	ToolChoice       any            `json:"tool_choice,omitempty"`
	PresencePenalty  float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64        `json:"frequency_penalty,omitempty"`
	NumChoices       uint64         `json:"n,omitempty"`
	Prediction       *Content       `json:"prediction,omitempty"`
	SafePrompt       bool           `json:"safe_prompt,omitempty"`
}

func (mistral *Client) ChatCompletion(ctx context.Context, context llm.Context, opts ...llm.Opt) (*Response, error) {
	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Append the system prompt at the beginning
	seq := make([]*MessageMeta, 0, len(context.(*session).seq)+1)
	if system := opt.SystemPrompt(); system != "" {
		seq = append(seq, systemPrompt(system))
	}
	seq = append(seq, context.(*session).seq...)

	// Request
	req, err := client.NewJSONRequest(reqChatCompletion{
		Model:            context.(*session).model.Name(),
		Temperature:      optTemperature(opt),
		TopP:             optTopP(opt),
		MaxTokens:        optMaxTokens(opt),
		Stream:           optStream(opt),
		StopSequences:    optStopSequences(opt),
		Seed:             optSeed(opt),
		Messages:         seq,
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

	// Response
	var response Response
	if err := mistral.DoWithContext(ctx, req, &response, client.OptPath("chat", "completions")); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}
