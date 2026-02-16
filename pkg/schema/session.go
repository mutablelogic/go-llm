package schema

import (
	"context"
	"fmt"
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// CONSTANTS

// DefaultMaxIterations is the default maximum number of tool-calling iterations
// per chat turn.
const DefaultMaxIterations = 10

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Conversation is a sequence of messages exchanged with an LLM
type Conversation []*Message

// Session represents a stored conversation with an LLM.
type Session struct {
	ID string `json:"id"`
	SessionMeta
	Messages Conversation `json:"messages,omitempty"`
	Overhead uint         `json:"overhead,omitempty"` // Constant token cost per turn (tools, system prompt)
	Created  time.Time    `json:"created"`
	Modified time.Time    `json:"modified"`
}

// SessionStore is the interface for session storage backends.
type SessionStore interface {
	// CreateSession creates a new session from the given metadata,
	// returning the session with a unique ID assigned.
	CreateSession(ctx context.Context, meta SessionMeta) (*Session, error)

	// GetSession retrieves an existing session by ID.
	// Returns an error if the session does not exist.
	GetSession(ctx context.Context, id string) (*Session, error)

	// ListSessions returns sessions matching the request, with pagination support.
	// Returns offset, limit and total count in the response.
	ListSessions(ctx context.Context, req ListSessionRequest) (*ListSessionResponse, error)

	// DeleteSession removes a session by ID.
	// Returns an error if the session does not exist.
	DeleteSession(ctx context.Context, id string) error

	// UpdateSession applies non-zero fields from the given metadata to an existing session.
	// Returns the updated session or an error if the session does not exist.
	UpdateSession(ctx context.Context, id string, meta SessionMeta) (*Session, error)

	// WriteSession persists the current state of a session.
	WriteSession(s *Session) error
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONVERSATION

// Append adds a message to the conversation
func (s *Conversation) Append(message Message) {
	*s = append(*s, &message)
}

// AppendWithOutput adds a message to the conversation, attributing token
// counts to individual messages. The last message in the conversation
// (typically the just-appended user message) receives an estimated token
// count based on its content rather than absorbing overhead such as tool
// schemas and system prompts. The response message receives the actual
// output token count from the provider.
func (s *Conversation) AppendWithOuput(message Message, input, output uint) {
	// Estimate tokens for the last message (the user message just appended
	// by WithSession) so it reflects only its content cost.
	if n := len(*s); n > 0 && (*s)[n-1].Tokens == 0 {
		(*s)[n-1].Tokens = (*s)[n-1].EstimateTokens()
	}

	// Filter out empty content blocks — some providers (notably Gemini)
	// produce empty text parts during streaming which are rejected when
	// sent back as context.
	filtered := message.Content[:0]
	for _, block := range message.Content {
		if block.Text != nil && *block.Text == "" {
			continue
		}
		filtered = append(filtered, block)
	}
	message.Content = filtered

	// Set the output tokens on the response message
	message.Tokens = output

	// Append the message
	*s = append(*s, &message)
}

// Return the total number of tokens in the conversation
func (s Conversation) Tokens() uint {
	total := uint(0)
	for _, msg := range s {
		total += msg.Tokens
	}
	return total
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SESSION

// Append adds a message to the session and updates the modified timestamp.
func (s *Session) Append(message Message) {
	s.Messages.Append(message)
	s.Modified = time.Now()
}

// Tokens returns the total token count across all messages.
func (s *Session) Tokens() uint {
	return s.Messages.Tokens()
}

// Conversation returns a pointer to the underlying message slice,
// compatible with generator.WithSession.
func (s *Session) Conversation() *Conversation {
	return &s.Messages
}

// Validate returns an error if the session is missing required fields.
func (s *Session) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("session id is required")
	}
	if s.Model == "" {
		return fmt.Errorf("session model is required")
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (s Conversation) String() string {
	return types.Stringify(s)
}

func (s *Session) String() string {
	return types.Stringify(s)
}
