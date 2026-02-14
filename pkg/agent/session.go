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
func (m *Manager) GetSession(ctx context.Context, req schema.GetSessionRequest) (*schema.Session, error) {
	return m.store.Get(ctx, req.ID)
}

// DeleteSession deletes a session by ID and returns it.
func (m *Manager) DeleteSession(ctx context.Context, req schema.DeleteSessionRequest) (*schema.Session, error) {
	s, err := m.store.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := m.store.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	return s, nil
}

// ListSessions returns sessions with pagination support.
func (m *Manager) ListSessions(ctx context.Context, req schema.ListSessionRequest) (*schema.ListSessionResponse, error) {
	return m.store.List(ctx, req)
}
