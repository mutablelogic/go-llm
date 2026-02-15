package agent

import (
	"context"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateSession creates a new session for the given model.
func (m *Manager) CreateSession(ctx context.Context, meta schema.SessionMeta) (*schema.Session, error) {
	// Resolve the model to ensure it exists, and fill in the provider if not set
	model, err := m.getModel(ctx, meta.Provider, meta.Model)
	if err != nil {
		return nil, err
	} else {
		meta.Provider = model.OwnedBy
	}

	// Create the session and return it
	return m.store.Create(ctx, meta)
}

// GetSession retrieves a session by ID.
func (m *Manager) GetSession(ctx context.Context, session string) (*schema.Session, error) {
	return m.store.Get(ctx, session)
}

// DeleteSession deletes a session by ID and returns it.
func (m *Manager) DeleteSession(ctx context.Context, session string) (*schema.Session, error) {
	s, err := m.store.Get(ctx, session)
	if err != nil {
		return nil, err
	}
	if err := m.store.Delete(ctx, session); err != nil {
		return nil, err
	}
	return s, nil
}

// ListSessions returns sessions with pagination support.
func (m *Manager) ListSessions(ctx context.Context, req schema.ListSessionRequest) (*schema.ListSessionResponse, error) {
	return m.store.List(ctx, req)
}

// UpdateSession updates a session's metadata. If Model or Provider are changed,
// they are validated against the registered providers first.
func (m *Manager) UpdateSession(ctx context.Context, id string, meta schema.SessionMeta) (*schema.Session, error) {
	// Retrieve existing session
	session, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// If model or provider is being changed, validate
	newModel := meta.Model
	newProvider := meta.Provider
	if newModel == "" {
		newModel = session.Model
	}
	if newProvider == "" {
		newProvider = session.Provider
	}
	if newModel != session.Model || newProvider != session.Provider {
		model, err := m.getModel(ctx, newProvider, newModel)
		if err != nil {
			return nil, err
		}
		session.Model = model.Name
		session.Provider = model.OwnedBy
	}

	// Apply non-zero fields
	if meta.Name != "" {
		session.Name = meta.Name
	}
	if meta.SystemPrompt != "" {
		session.SystemPrompt = meta.SystemPrompt
	}
	if meta.Format != nil {
		session.Format = meta.Format
	}
	if meta.Thinking {
		session.Thinking = true
	}
	if meta.ThinkingBudget > 0 {
		session.ThinkingBudget = meta.ThinkingBudget
	}

	// Persist
	if err := m.store.Write(session); err != nil {
		return nil, err
	}
	return session, nil
}
