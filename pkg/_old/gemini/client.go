/*
gemini implements an API client for Google's Gemini LLM
https://ai.google.dev/gemini-api/docs
*/
package gemini

import (
	// Packages
	"context"

	llm "github.com/mutablelogic/go-llm"
	genai "google.golang.org/genai"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*genai.Client
	cache map[string]llm.Model
}

var _ llm.Agent = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultName = "gemini"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new client
func New(ApiKey string) (*Client, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{client, nil}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the name of the agent
func (Client) Name() string {
	return defaultName
}
