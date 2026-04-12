package schema

import (
	// Packages
	uuid "github.com/google/uuid"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ChatRequest contains the core fields of a chat request without attachments.
type ChatRequest struct {
	Session       uuid.UUID `json:"session" help:"Session ID"`
	Text          string    `json:"text" arg:"" help:"User input text"`
	Tools         []string  `json:"tools,omitzero" help:"Tool names to include (nil means all, empty means none)" optional:""`
	MaxIterations uint      `json:"max_iterations,omitempty" help:"Maximum tool-calling iterations (0 uses default)" optional:""`
	SystemPrompt  string    `json:"system_prompt,omitempty" help:"Per-request system prompt appended to the session prompt" optional:""`
}

// SessionChannelRequest represents one inbound channel frame for a session.
// The session is selected by the path parameter, not the frame body.
type SessionChannelRequest struct {
	Text          string   `json:"text" arg:"" help:"User input text"`
	Tools         []string `json:"tools,omitzero" help:"Tool names to include (nil means all, empty means none)" optional:""`
	MaxIterations uint     `json:"max_iterations,omitempty" help:"Maximum tool-calling iterations (0 uses default)" optional:""`
	SystemPrompt  string   `json:"system_prompt,omitempty" help:"Per-request system prompt appended to the session prompt" optional:""`
}

// ChatResponse represents the response from a chat request.
type ChatResponse struct {
	ID      uint64    `json:"id,omitempty" help:"Persisted message row ID for the final reply when available" example:"42"`
	Session uuid.UUID `json:"session,omitzero" help:"Session owning the final reply when available" optional:""`
	CompletionResponse
	Usage *UsageMeta `json:"usage,omitempty"`
}
