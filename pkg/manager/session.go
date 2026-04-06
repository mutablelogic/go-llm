package manager

import (
	"context"
	"errors"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	uuid "github.com/google/uuid"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateSession validates and persists a new session for the authenticated user.
// If Parent is set, the parent session must exist, belong to the same user,
// and its generator settings are used as defaults for the child session.
// If user is nil, it creates a user-less session.
func (m *Manager) CreateSession(ctx context.Context, req schema.SessionInsert, user *auth.User) (_ *schema.Session, err error) {
	// OTel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateSession",
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// All work runs in a single transaction: parent fetch + ownership check +
	// meta merge + provider/model resolution + insert.
	var result schema.Session
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		conn = conn.With("user", user.UUID())

		// If a parent session is provided, fetch it (no user filter — we check
		// ownership explicitly), then merge generator settings as defaults.
		if req.Parent != uuid.Nil {
			var parent schema.Session
			if err := conn.With("user", uuid.UUID{}).Get(ctx, &parent, schema.SessionIDSelector(req.Parent)); err != nil {
				return normalizeSessionError(req.Parent, err)
			}
			if parent.User != user.UUID() {
				return httpresponse.ErrForbidden.With("parent session belongs to another user")
			}
			req.GeneratorMeta = parent.GeneratorMeta.MergeFrom(req.GeneratorMeta)
		}

		// Resolve provider and model (read-only, safe inside transaction).
		provider, model, _, _, err := m.generatorFromMeta(ctx, req.GeneratorMeta, user, generationContextChat)
		if err != nil {
			return err
		}
		req.Provider = types.Ptr(provider.Name)
		req.Model = types.Ptr(model.Name)

		return conn.Insert(ctx, &result, req)
	}); err != nil {
		return nil, pg.NormalizeError(err)
	}

	// Return success
	return types.Ptr(result), nil
}

// GetSession returns a session by ID. If user is non-nil, the session must be
// owned by that user; otherwise ErrForbidden is returned.
func (m *Manager) GetSession(ctx context.Context, session uuid.UUID, user *auth.User) (_ *schema.Session, err error) {
	// OTel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "GetSession",
		attribute.String("id", session.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Get the session - if user is provided, ensure session belongs to that user.
	var result schema.Session
	if err := m.PoolConn.With("user", user.UUID()).Get(ctx, &result, schema.SessionIDSelector(session)); err != nil {
		return nil, normalizeSessionError(session, err)
	}

	// Return success
	return types.Ptr(result), nil
}

// UpdateSession updates the metadata for a session and returns the updated session.
// If user is non-nil, the session must be owned by that user.
// The incoming GeneratorMeta is merged over the existing one (incoming fields win).
func (m *Manager) UpdateSession(ctx context.Context, session uuid.UUID, meta schema.SessionMeta, user *auth.User) (_ *schema.Session, err error) {
	// OTel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "UpdateSession",
		attribute.String("id", session.String()),
		attribute.String("meta", meta.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Fetch, merge GeneratorMeta, and update in one transaction.
	var result schema.Session
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		conn = conn.With("user", user.UUID())
		var existing schema.Session
		if err := conn.Get(ctx, &existing, schema.SessionIDSelector(session)); err != nil {
			return normalizeSessionError(session, err)
		}
		// Only merge GeneratorMeta from the existing session if the caller
		// supplied at least one generator field; otherwise leave it nil so
		// the Update binding detects "no fields to update".
		if !meta.GeneratorMeta.IsZero() {
			meta.GeneratorMeta = existing.GeneratorMeta.MergeFrom(meta.GeneratorMeta)

			// If provider or model changed, validate the merged generator resolves
			// to exactly one accessible model before committing.
			if meta.GeneratorMeta.Provider != existing.GeneratorMeta.Provider ||
				meta.GeneratorMeta.Model != existing.GeneratorMeta.Model {
				if _, _, _, _, err := m.generatorFromMeta(ctx, meta.GeneratorMeta, user, generationContextChat); err != nil {
					return err
				}
			}
		}
		return conn.Update(ctx, &result, schema.SessionIDSelector(session), meta)
	}); err != nil {
		return nil, normalizeSessionError(session, err)
	}

	// Return success
	return types.Ptr(result), nil
}

// DeleteSession removes a session by ID and returns the deleted session.
// If user is non-nil, the session must be owned by that user.
func (m *Manager) DeleteSession(ctx context.Context, session uuid.UUID, user *auth.User) (_ *schema.Session, err error) {
	// OTel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "DeleteSession",
		attribute.String("id", session.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Delete the session - if user is provided, ensure session belongs to that user.
	var result schema.Session
	if err := m.PoolConn.With("user", user.UUID()).Delete(ctx, &result, schema.SessionIDSelector(session)); err != nil {
		return nil, normalizeSessionError(session, err)
	}

	// Return success
	return types.Ptr(result), nil
}

// ListSessions returns a paginated list of sessions matching the request.
// If user is non-nil, only sessions owned by that user are returned.
func (m *Manager) ListSessions(ctx context.Context, req schema.SessionListRequest, user *auth.User) (_ *schema.SessionList, err error) {
	// OTel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListSessions",
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	result := schema.SessionList{SessionListRequest: req}
	if err := m.PoolConn.With("user", user.UUID()).List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit.Clamp(uint64(result.Count))

	return types.Ptr(result), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func normalizeSessionError(session uuid.UUID, err error) error {
	err = pg.NormalizeError(err)
	if errors.Is(err, pg.ErrNotFound) || errors.Is(err, schema.ErrNotFound) {
		return schema.ErrNotFound.Withf("session %q", session)
	}
	return err
}
