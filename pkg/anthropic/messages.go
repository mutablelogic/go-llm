package anthropic

import (
	"context"
	"encoding/json"
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

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

type reqMessages struct {
	Model    string         `json:"model"`
	Messages []*MessageMeta `json:"messages"`
	*opt
}

func (anthropic *Client) Messages(ctx context.Context, model llm.Model, context llm.Context, opts ...llm.Opt) (*Response, error) {
	// Apply options
	opt, err := apply(opts...)
	if err != nil {
		return nil, err
	}

	// Context to append to the request
	messages := []*MessageMeta{}
	message, ok := context.(*message)
	if !ok || message == nil {
		return nil, llm.ErrBadParameter.With("incompatible context")
	} else if message.Role() != "user" {
		return nil, llm.ErrBadParameter.Withf("invalid role, %q", message.Role())
	} else {
		messages = append(messages, &message.MessageMeta)
	}

	// Set max_tokens
	if opt.MaxTokens == 0 {
		opt.MaxTokens = defaultMaxTokens(model.Name())
	}

	// Request
	req, err := client.NewJSONRequest(reqMessages{
		Model:    model.Name(),
		Messages: messages,
		opt:      opt,
	})
	if err != nil {
		return nil, err
	}

	// Response
	var response Response
	if err := anthropic.DoWithContext(ctx, req, &response, client.OptPath("messages")); err != nil {
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
