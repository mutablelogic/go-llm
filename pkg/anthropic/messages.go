package anthropic

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type messagesRequest struct {
	MaxTokens int             `json:"max_tokens,omitempty"`
	Messages  *schema.Session `json:"messages"`
	Model     string          `json:"model"`
}

type messagesResponse struct {
	Id    string `json:"id"`
	Model string `json:"model"`
	Type  string `json:"type"`
	schema.Message
	StopReason    string  `json:"stop_reason"` // "end_turn", "max_tokens", "stop_sequence", "tool_use", "pause_turn", "refusal"
	StopSequence  *string `json:"stop_sequence,omitempty"`
	messagesUsage `json:"usage"`
}

type messagesUsage struct {
	InputTokens  uint   `json:"input_tokens"`
	OutputTokens uint   `json:"output_tokens"`
	ServiceTier  string `json:"service_tier"` // standard, priority, batch
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultMaxTokens = 1024
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Messages provides the next message in a session, and updates the session with the response
func (anthropic *Client) Messages(ctx context.Context, model string, session *schema.Session) (*schema.Message, error) {

	// Create a request
	request, err := client.NewJSONRequest(messagesRequest{
		MaxTokens: defaultMaxTokens,
		Messages:  session,
		Model:     model,
	})
	if err != nil {
		return nil, err
	}

	// Send the request
	var response messagesResponse
	if err := anthropic.DoWithContext(ctx, request, &response, client.OptPath("messages")); err != nil {
		return nil, err
	}

	// Append the message to the session
	session.AppendWithOuput(response.Message, response.InputTokens, response.OutputTokens)

	// Return success
	return &response.Message, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r messagesResponse) String() string {
	return schema.Stringify(r)
}
