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
	*Metrics    `json:"usage,omitempty"`
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

type reqChatCompletion struct {
	Model            string           `json:"model"`
	Temperature      float64          `json:"temperature,omitempty"`
	TopP             float64          `json:"top_p,omitempty"`
	MaxTokens        uint64           `json:"max_tokens,omitempty"`
	Stream           bool             `json:"stream,omitempty"`
	StopSequences    []string         `json:"stop,omitempty"`
	Seed             uint64           `json:"random_seed,omitempty"`
	Format           any              `json:"response_format,omitempty"`
	Tools            []llm.Tool       `json:"tools,omitempty"`
	ToolChoice       any              `json:"tool_choice,omitempty"`
	PresencePenalty  float64          `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64          `json:"frequency_penalty,omitempty"`
	NumCompletions   uint64           `json:"n,omitempty"`
	Prediction       *Content         `json:"prediction,omitempty"`
	SafePrompt       bool             `json:"safe_prompt,omitempty"`
	Messages         []llm.Completion `json:"messages"`
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
	req, err := client.NewJSONRequest(reqChatCompletion{
		Model:            model.Name(),
		Temperature:      optTemperature(opt),
		TopP:             optTopP(opt),
		MaxTokens:        optMaxTokens(opt),
		Stream:           optStream(opt),
		StopSequences:    optStopSequences(opt),
		Seed:             optSeed(opt),
		Format:           optFormat(opt),
		Tools:            optTools(model.Client, opt),
		ToolChoice:       optToolChoice(opt),
		PresencePenalty:  optPresencePenalty(opt),
		FrequencyPenalty: optFrequencyPenalty(opt),
		NumCompletions:   optNumCompletions(opt),
		Prediction:       optPrediction(opt),
		SafePrompt:       optSafePrompt(opt),
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

	// Append tool calls
	for i := range c.Delta.Calls {
		if i >= len(message.Calls) {
			message.Calls = append(message.Calls, toolcall{})
		}
	}

	for i, call := range c.Delta.Calls {
		if call.meta.Id != "" {
			message.Calls[i].meta.Id = call.meta.Id
		}
		if call.meta.Index != 0 {
			message.Calls[i].meta.Index = call.meta.Index
		}
		if call.meta.Type != "" {
			message.Calls[i].meta.Type = call.meta.Type
		}
		if call.meta.Function.Name != "" {
			message.Calls[i].meta.Function.Name = call.meta.Function.Name
		}
		if call.meta.Function.Arguments != "" {
			message.Calls[i].meta.Function.Arguments += call.meta.Function.Arguments
		}
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
