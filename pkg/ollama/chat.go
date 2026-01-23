package ollama

import (
	"context"
	"encoding/json"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type chatRequest struct {
	Messages []schema.OllamaMessage `json:"messages"`
	Model    string                 `json:"model"`
}

type chatResponse struct {
	Model                string `json:"model"`
	schema.OllamaMessage `json:"message"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Generate the next chat message in a conversation between a user and an assistant.
// https://docs.ollama.com/api/chat
func (ollama *Client) Chat(ctx context.Context, model string, session *schema.Session) (*schema.Message, error) {
	// Convert session messages to Ollama format
	ollamaMessages := make([]schema.OllamaMessage, 0, len(*session))
	for _, msg := range *session {
		ollamaMessages = append(ollamaMessages, schema.OllamaMessage{Message: *msg})
	}

	// Create a request
	request, err := client.NewJSONRequest(chatRequest{
		Messages: ollamaMessages,
		Model:    model,
	})
	if err != nil {
		return nil, err
	}

	// Send the request
	var response chatResponse
	if err := ollama.DoWithContext(ctx, request, &response, client.OptPath("chat")); err != nil {
		return nil, err
	}

	// Append the message to the session
	session.Append(response.OllamaMessage.Message)

	// Return success
	return &response.OllamaMessage.Message, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r chatResponse) String() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}
