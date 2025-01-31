package anthropic

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

// Messages Response
type Response struct {
	Type  string `json:"type"`
	Model string `json:"model"`
	Id    string `json:"id"`
	MessageMeta
	Reason       string  `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
	Metrics      `json:"usage,omitempty"`
}

// Metrics
type Metrics struct {
	CacheCreationInputTokens uint `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     uint `json:"cache_read_input_tokens,omitempty"`
	InputTokens              uint `json:"input_tokens,omitempty"`
	OutputTokens             uint `json:"output_tokens,omitempty"`
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

func (r opt) String() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

type reqMessages struct {
	Model    string         `json:"model"`
	Messages []*MessageMeta `json:"messages"`
	Tools    []llm.Tool     `json:"tools,omitempty"`
	opt
}

func (anthropic *Client) Messages(ctx context.Context, context llm.Context, opts ...llm.Opt) (*Response, error) {
	// Apply options
	opt, err := apply(opts...)
	if err != nil {
		return nil, err
	}

	// Set max_tokens
	if opt.MaxTokens == 0 {
		opt.MaxTokens = defaultMaxTokens(context.(*session).model.Name())
	}

	// Request
	req, err := client.NewJSONRequest(reqMessages{
		Model:    context.(*session).model.Name(),
		Messages: context.(*session).seq,
		Tools:    opt.tools(anthropic),
		opt:      *opt,
	})
	if err != nil {
		return nil, err
	}

	// Stream
	var response Response
	reqopts := []client.RequestOpt{
		client.OptPath("messages"),
	}
	if opt.Stream {
		// Append delta to content
		appendDelta := func(content []*Content, delta *Content) ([]*Content, error) {
			if len(content) == 0 {
				return nil, fmt.Errorf("unexpected delta")
			}

			// Get the content block we want to append to
			last := content[len(content)-1]

			// Append text_delta
			switch {
			case last.Type == "text" && delta.Type == "text_delta":
				last.Text += delta.Text
			case last.Type == "tool_use" && delta.Type == "input_json_delta":
				last.InputJson += delta.InputJson
			default:
				return nil, fmt.Errorf("unexpected delta %s for %s", delta.Type, last.Type)
			}

			// Return the content
			return content, nil
		}
		reqopts = append(reqopts, client.OptTextStreamCallback(func(evt client.TextStreamEvent) error {
			switch evt.Event {
			case "message_start":
				// Start of a message
				var r struct {
					Type     string   `json:"type"`
					Response Response `json:"message"`
				}
				if err := evt.Json(&r); err != nil {
					return err
				} else {
					response = r.Response
				}
			case "content_block_start":
				// Start of a content block, append to response
				var r struct {
					Type    string  `json:"type"`
					Index   uint    `json:"index"`
					Content Content `json:"content_block"`
				}
				if err := evt.Json(&r); err != nil {
					return err
				} else if int(r.Index) != len(response.MessageMeta.Content) {
					return fmt.Errorf("%s: unexpected index %d", r.Type, r.Index)
				} else {
					response.MessageMeta.Content = append(response.MessageMeta.Content, &r.Content)
				}
			case "content_block_delta":
				// Continuation of a content block, append to content
				var r struct {
					Type    string  `json:"type"`
					Index   uint    `json:"index"`
					Content Content `json:"delta"`
				}
				if err := evt.Json(&r); err != nil {
					return err
				} else if int(r.Index) != len(response.MessageMeta.Content)-1 {
					fmt.Println(response.MessageMeta)
					return fmt.Errorf("%s: unexpected index %d", r.Type, r.Index)
				} else if content, err := appendDelta(response.MessageMeta.Content, &r.Content); err != nil {
					return err
				} else {
					response.MessageMeta.Content = content
				}
			case "content_block_stop":
				// End of a content block
				var r struct {
					Type  string `json:"type"`
					Index uint   `json:"index"`
				}
				if err := evt.Json(&r); err != nil {
					return err
				} else if int(r.Index) != len(response.MessageMeta.Content)-1 {
					return fmt.Errorf("%s: unexpected index %d", r.Type, r.Index)
				}
				// We need to convert the partial_json response into a full json object
				content := response.MessageMeta.Content[r.Index]
				if content.Type == "tool_use" && content.InputJson != "" {
					if err := json.Unmarshal([]byte(content.InputJson), &content.Input); err != nil {
						return err
					}
				}
			case "message_delta":
				// Message update
				var r struct {
					Type  string   `json:"type"`
					Delta Response `json:"delta"`
					Usage Metrics  `json:"usage"`
				}
				if err := evt.Json(&r); err != nil {
					return err
				}

				// Update stop reason
				response.Reason = r.Delta.Reason
				response.StopSequence = r.Delta.StopSequence

				// Update metrics
				response.Metrics.InputTokens += r.Usage.InputTokens
				response.Metrics.OutputTokens += r.Usage.OutputTokens
				response.Metrics.CacheCreationInputTokens += r.Usage.CacheCreationInputTokens
				response.Metrics.CacheReadInputTokens += r.Usage.CacheReadInputTokens
			case "message_stop":
				// NO-OP
				return nil
			case "ping":
				// NO-OP
				return nil
			default:
				// NO-OP
				return nil
			}

			if opt.callback != nil {
				opt.callback(&response)
			}

			// Return success
			return nil
		}))
	}

	// Response
	if err := anthropic.DoWithContext(ctx, req, &response, reqopts...); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func defaultMaxTokens(model string) uint {
	// https://docs.anthropic.com/en/docs/about-claude/models
	switch {
	case strings.Contains(model, "claude-3-5-haiku"):
		return 8192
	case strings.Contains(model, "claude-3-5-sonnet"):
		return 8192
	default:
		return 4096
	}
}
