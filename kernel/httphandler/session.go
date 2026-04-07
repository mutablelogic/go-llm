package httphandler

import (
	"context"
	"net/http"

	// Packages
	middleware "github.com/djthorpe/go-auth/pkg/middleware"
	uuid "github.com/google/uuid"
	llmmanager "github.com/mutablelogic/go-llm/kernel/manager"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func SessionHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "session", nil, httprequest.NewPathItem(
		"Session operations",
		"List and create operations on sessions",
		"Sessions",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = listSessions(r.Context(), manager, w, r)
		},
		"List sessions",
		opts.WithQuery(jsonschema.MustFor[schema.SessionListRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.SessionList]()),
		opts.WithErrorResponse(400, "Invalid request parameters."),
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = createSession(r.Context(), manager, w, r)
		},
		"Create session",
		opts.WithJSONRequest(jsonschema.MustFor[schema.SessionInsert]()),
		opts.WithJSONResponse(201, jsonschema.MustFor[schema.Session]()),
		opts.WithErrorResponse(400, "Invalid request body or session creation failure."),
		opts.WithErrorResponse(403, "Parent session belongs to another user."),
		opts.WithErrorResponse(404, "Parent session, model, or provider not found."),
		opts.WithErrorResponse(409, "Multiple models matched; specify a provider."),
	)
}

func SessionResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "session/{session}", jsonschema.MustFor[schema.SessionIDSelector](), httprequest.NewPathItem(
		"Session operations",
		"Get, update, and delete operations on a session",
		"Sessions",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getSession(r.Context(), manager, w, r)
		},
		"Get session",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Session]()),
		opts.WithErrorResponse(400, "Invalid session ID."),
		opts.WithErrorResponse(404, "Session not found."),
	).Patch(
		func(w http.ResponseWriter, r *http.Request) {
			_ = updateSession(r.Context(), manager, w, r)
		},
		"Update session",
		opts.WithJSONRequest(jsonschema.MustFor[schema.SessionMeta]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Session]()),
		opts.WithErrorResponse(400, "Invalid request body or session ID."),
		opts.WithErrorResponse(404, "Session not found."),
	).Delete(
		func(w http.ResponseWriter, r *http.Request) {
			_ = deleteSession(r.Context(), manager, w, r)
		},
		"Delete session",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Session]()),
		opts.WithErrorResponse(400, "Invalid session ID."),
		opts.WithErrorResponse(404, "Session not found."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func listSessions(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.SessionListRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	if sessions, err := manager.ListSessions(ctx, req, middleware.UserFromContext(ctx)); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	} else {
		return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), sessions)
	}
}

func createSession(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.SessionInsert
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	session, err := manager.CreateSession(ctx, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), session)
}

func getSession(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("session"))
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	session, err := manager.GetSession(ctx, id, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
}

func deleteSession(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("session"))
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	session, err := manager.DeleteSession(ctx, id, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
}

func updateSession(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("session"))
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	var meta schema.SessionMeta
	if err := httprequest.Read(r, &meta); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	session, err := manager.UpdateSession(ctx, id, meta, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
}
