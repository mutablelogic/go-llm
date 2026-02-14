package agent

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateSession creates a new session for the given model.
func (m *Manager) CreateSession(ctx context.Context, req schema.CreateSessionRequest) (*schema.Session, error) {
	if m.store == nil {
		return nil, llm.ErrNotImplemented.With("no session store configured")
	}

	// Resolve the model
	model, err := m.getModel(ctx, req.Provider, req.Model)
	if err != nil {
		return nil, err
	}

	return m.store.Create(ctx, req.Name, *model)
}

// GetSession retrieves a session by ID.
func (m *Manager) GetSession(ctx context.Context, req schema.GetSessionRequest) (*schema.Session, error) {
	if m.store == nil {
		return nil, llm.ErrNotImplemented.With("no session store configured")
	}
	return m.store.Get(ctx, req.ID)
}

// DeleteSession deletes a session by ID.
func (m *Manager) DeleteSession(ctx context.Context, req schema.DeleteSessionRequest) error {
	if m.store == nil {
		return llm.ErrNotImplemented.With("no session store configured")
	}
	return m.store.Delete(ctx, req.ID)
}

// ListSessions returns sessions with pagination support.
func (m *Manager) ListSessions(ctx context.Context, req schema.ListSessionsRequest) (*schema.ListSessionsResponse, error) {
	if m.store == nil {
		return nil, llm.ErrNotImplemented.With("no session store configured")
	}

	// Fetch all sessions from the store
	all, err := m.store.List(ctx)
	if err != nil {
		return nil, err
	}

	// Paginate
	total := uint(len(all))
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + req.Limit
	if req.Limit == 0 || end > total {
		end = total
	}

	return &schema.ListSessionsResponse{
		Count:  total,
		Offset: req.Offset,
		Limit:  req.Limit,
		Body:   all[start:end],
	}, nil
}
