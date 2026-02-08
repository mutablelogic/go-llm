package mistral

import (
	"context"
	"encoding/json"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type chatRequest struct {
	Model            string                  `json:"model"`
	Messages         []schema.MistralMessage `json:"messages"`
	Temperature      *float64                `json:"temperature,omitempty"`
	TopP             *float64                `json:"top_p,omitempty"`
	MaxTokens        *int                    `json:"max_tokens,omitempty"`
	Stop             []string                `json:"stop,omitempty"`
	Stream           bool                    `json:"stream,omitempty"`
	RandomSeed       *uint                   `json:"random_seed,omitempty"`
	Tools            []json.RawMessage       `json:"tools,omitempty"`
	ToolChoice       any                     `json:"tool_choice,omitempty"`
	PresencePenalty  *float64                `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64                `json:"frequency_penalty,omitempty"`
	NumChoices       *int                    `json:"n,omitempty"`
	SafePrompt       bool                    `json:"safe_prompt,omitempty"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Index        int                   `json:"index"`
	Message      schema.MistralMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Chat generates the next message in a conversation and appends it to the session.
func (c *Client) Chat(ctx context.Context, model string, session *schema.Session, opts ...opt.Opt) (*schema.Message, error) {
	// Build request
	req, err := chatRequestFromOpts(model, session, opts...)
	if err != nil {
		return nil, err
	}

	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var resp chatResponse
	if err := c.DoWithContext(ctx, httpReq, &resp, client.OptPath("chat", "completions")); err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, llm.ErrNotFound.With("no choices returned")
	}

	msg := resp.Choices[0].Message.Message
	input := uint(resp.Usage.PromptTokens)
	output := uint(resp.Usage.CompletionTokens)
	session.AppendWithOuput(msg, input, output)

	return &msg, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func chatRequestFromOpts(model string, session *schema.Session, opts ...opt.Opt) (*chatRequest, error) {
	if session == nil {
		return nil, llm.ErrBadParameter.With("session is required")
	}
	if len(*session) == 0 {
		return nil, llm.ErrBadParameter.With("at least one message is required")
	}

	// Wrap messages for provider-specific marshaling. Mistral requires exactly
	// one tool_result per message (identified by tool_call_id), so we split any
	// multi-result message into separate messages to keep the call/response
	// counts aligned.
	msgs := make([]schema.MistralMessage, 0, len(*session))
	for _, msg := range *session {
		if hasToolResult(msg.Content) {
			for _, block := range msg.Content {
				if block.Type != schema.ContentTypeToolResult {
					continue
				}
				msgs = append(msgs, schema.MistralMessage{Message: schema.Message{
					Role:    schema.MessageRoleTool,
					Content: []schema.ContentBlock{block},
				}})
			}
			continue
		}

		// Copy non-tool messages so we can safely adjust roles without mutating
		// shared session history used by other providers.
		copyMsg := *msg
		msgs = append(msgs, schema.MistralMessage{Message: copyMsg})
	}

	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	if o.GetBool("stream") {
		return nil, llm.ErrNotImplemented.With("streaming chat is not implemented for mistral")
	}

	req := &chatRequest{
		Model:      model,
		Messages:   msgs,
		Stop:       o.GetStringArray("stop_sequences"),
		SafePrompt: o.GetBool("safe_prompt"),
	}

	if o.Has("temperature") {
		v := o.GetFloat64("temperature")
		req.Temperature = &v
	}
	if o.Has("top_p") {
		v := o.GetFloat64("top_p")
		req.TopP = &v
	}
	if o.Has("max_tokens") {
		v := int(o.GetUint("max_tokens"))
		req.MaxTokens = &v
	}
	if o.Has("random_seed") {
		v := o.GetUint("random_seed")
		req.RandomSeed = &v
	}
	if o.Has("presence_penalty") {
		v := o.GetFloat64("presence_penalty")
		req.PresencePenalty = &v
	}
	if o.Has("frequency_penalty") {
		v := o.GetFloat64("frequency_penalty")
		req.FrequencyPenalty = &v
	}
	if o.Has("num_completions") {
		v := int(o.GetUint("num_completions"))
		req.NumChoices = &v
	} else if o.Has("n") {
		v := int(o.GetUint("n"))
		req.NumChoices = &v
	}

	// Tools
	for _, toolJSON := range o.GetStringArray("tools") {
		req.Tools = append(req.Tools, json.RawMessage(toolJSON))
	}
	if tc := o.GetString("tool_choice"); tc != "" {
		req.ToolChoice = tc
	}

	return req, nil
}

// hasToolResult reports whether any content block is a tool_result.
func hasToolResult(blocks []schema.ContentBlock) bool {
	for _, b := range blocks {
		if b.Type == schema.ContentTypeToolResult {
			return true
		}
	}
	return false
}
