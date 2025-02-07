package mistral

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	"github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Chat Completion Response
type Response struct {
	Id          string `json:"id"`
	Type        string `json:"object"`
	Created     uint64 `json:"created"`
	Model       string `json:"model"`
	Completions `json:"choices"`
	Metrics     `json:"usage,omitempty"`
}

// Possible completions
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
	InputTokens  uint64 `json:"prompt_tokens,omitempty"`
	OutputTokens uint   `json:"completion_tokens,omitempty"`
	TotalTokens  uint   `json:"total_tokens,omitempty"`
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

type reqChatCompletion struct {
	Model            string     `json:"model"`
	Temperature      float64    `json:"temperature,omitempty"`
	TopP             float64    `json:"top_p,omitempty"`
	MaxTokens        uint64     `json:"max_tokens,omitempty"`
	Stream           bool       `json:"stream,omitempty"`
	StopSequences    []string   `json:"stop,omitempty"`
	Seed             uint64     `json:"random_seed,omitempty"`
	Messages         []*Message `json:"messages"`
	Format           any        `json:"response_format,omitempty"`
	Tools            []llm.Tool `json:"tools,omitempty"`
	ToolChoice       any        `json:"tool_choice,omitempty"`
	PresencePenalty  float64    `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64    `json:"frequency_penalty,omitempty"`
	NumChoices       uint64     `json:"n,omitempty"`
	Prediction       *Content   `json:"prediction,omitempty"`
	SafePrompt       bool       `json:"safe_prompt,omitempty"`
}

func (model *model) Completion(ctx context.Context, prompt string, opts ...llm.Opt) (llm.Completion, error) {
	// TODO
	return nil, llm.ErrNotImplemented
}

func (mistral *Client) ChatCompletion(ctx context.Context, context llm.Context, opts ...llm.Opt) (*Response, error) {
	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Append the system prompt at the beginning
	messages := make([]*Message, 0, len(context.(*session).seq)+1)
	if system := opt.SystemPrompt(); system != "" {
		messages = append(messages, systemPrompt(system))
	}

	// Always append the first message of each completion
	for _, message := range context.(*session).seq {
		messages = append(messages, message)
	}

	// Request
	req, err := client.NewJSONRequest(reqChatCompletion{
		Model:            context.(*session).model.Name(),
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
// PUBLIC METHODS - COMPLETIONS

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

// Return the current session tool calls given the completion index.
// Will return nil if no tool calls were returned.
func (c Completions) ToolCalls(index int) []llm.ToolCall {
	if index < 0 || index >= len(c) {
		return nil
	}
	return c[index].Message.ToolCalls(0)
}
