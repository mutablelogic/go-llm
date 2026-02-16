package manager

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
	return m.sessionStore.CreateSession(ctx, meta)
}

// GetSession retrieves a session by ID.
func (m *Manager) GetSession(ctx context.Context, session string) (*schema.Session, error) {
	return m.sessionStore.GetSession(ctx, session)
}

// DeleteSession deletes a session by ID and returns it.
func (m *Manager) DeleteSession(ctx context.Context, session string) (*schema.Session, error) {
	s, err := m.sessionStore.GetSession(ctx, session)
	if err != nil {
		return nil, err
	}
	if err := m.sessionStore.DeleteSession(ctx, session); err != nil {
		return nil, err
	}
	return s, nil
}

// ListSessions returns sessions with pagination support.
func (m *Manager) ListSessions(ctx context.Context, req schema.ListSessionRequest) (*schema.ListSessionResponse, error) {
	return m.sessionStore.ListSessions(ctx, req)
}

// UpdateSession updates a session's metadata. If Model or Provider are changed,
// they are validated against the registered providers first.
func (m *Manager) UpdateSession(ctx context.Context, id string, meta schema.SessionMeta) (*schema.Session, error) {
	// If model or provider is being changed, validate
	if meta.Model != "" || meta.Provider != "" {
		model, err := m.getModel(ctx, meta.Provider, meta.Model)
		if err != nil {
			return nil, err
		}
		meta.Model = model.Name
		meta.Provider = model.OwnedBy
	}

	// Delegate to store
	return m.sessionStore.UpdateSession(ctx, id, meta)
}
