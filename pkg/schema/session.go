package schema

import (
	"context"
	"fmt"
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Conversation is a sequence of messages exchanged with an LLM
type Conversation []*Message

// Session represents a stored conversation with an LLM.
type Session struct {
	ID string `json:"id"`
	SessionMeta
	Messages Conversation `json:"messages,omitempty"`
	Created  time.Time    `json:"created"`
	Modified time.Time    `json:"modified"`
}

// Store is the interface for session storage backends.
type Store interface {
	// Create creates a new session from the given metadata,
	// returning the session with a unique ID assigned.
	Create(ctx context.Context, meta SessionMeta) (*Session, error)

	// Get retrieves an existing session by ID.
	// Returns an error if the session does not exist.
	Get(ctx context.Context, id string) (*Session, error)

	// List returns sessions matching the request, with pagination support.
	// Returns offset, limit and total count in the response.
	List(ctx context.Context, req ListSessionRequest) (*ListSessionResponse, error)

	// Delete removes a session by ID.
	// Returns an error if the session does not exist.
	Delete(ctx context.Context, id string) error

	// Write persists the current state of a session.
	Write(s *Session) error
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONVERSATION

// Append adds a message to the conversation
func (s *Conversation) Append(message Message) {
	*s = append(*s, &message)
}

// AppendWithOutput adds a message to the conversation, re-calculating token usage
// for the conversation
func (s *Conversation) AppendWithOuput(message Message, input, output uint) {
	// Calculate the input tokens and adjust the last message to account for the tokens
	tokens := uint(0)
	for _, msg := range *s {
		tokens += msg.Tokens
	}
	if input > tokens {
		(*s)[len(*s)-1].Tokens = input - tokens
	}

	// Set the output tokens
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
// compatible with agent.WithSession.
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
