package session

import (
	"context"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-llm/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Store is the interface for session storage backends.
type Store interface {
	// Create creates a new session with the given name and model,
	// returning the session with a unique ID assigned.
	Create(ctx context.Context, name string, model schema.Model) (*Session, error)

	// Get retrieves an existing session by ID.
	// Returns llm.ErrNotFound if the session does not exist.
	Get(ctx context.Context, id string) (*Session, error)

	// List returns all sessions, ordered by last modified time (most recent first).
	// Supports WithLimit to cap the number of results.
	List(ctx context.Context, opts ...opt.Opt) ([]*Session, error)

	// Delete removes a session by ID.
	// Returns llm.ErrNotFound if the session does not exist.
	Delete(ctx context.Context, id string) error

	// Write persists the current state of a session.
	Write(s *Session) error
}

// Session represents a stored conversation with an LLM.
type Session struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Model    schema.Model   `json:"model"`
	Messages schema.Session `json:"messages,omitempty"`
	Created  time.Time      `json:"created"`
	Modified time.Time      `json:"modified"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Append adds a message to the session and updates the modified timestamp.
func (s *Session) Append(message schema.Message) {
	s.Messages.Append(message)
	s.Modified = time.Now()
}

// Tokens returns the total token count across all messages.
func (s *Session) Tokens() uint {
	return s.Messages.Tokens()
}

// Session returns a pointer to the underlying message slice,
// compatible with agent.WithSession.
func (s *Session) MessageSession() *schema.Session {
	return &s.Messages
}

// Validate returns an error if the session is missing required fields.
func (s *Session) Validate() error {
	if s.ID == "" {
		return llm.ErrBadParameter.With("session id is required")
	}
	if s.Model.Name == "" {
		return llm.ErrBadParameter.With("session model is required")
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (s *Session) String() string {
	return types.Stringify(s)
}
