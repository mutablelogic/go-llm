package httphandler

import (
	"errors"
	"net/http"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sessionID is the path parameter of a session's identifier
type sessionID struct {
	ID uuid.UUID `json:"id" help:"Session identifier"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func sessionHandlers(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag("Sessions", documentation.Section(2, "Session Operations").Body)
	return errors.Join(
		sessionCreateListHandlers(r, router, documentation),
		sessionGetUpdateDeleteHandlers(r, router, documentation),
	)
}

func sessionCreateListHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/session", nil, func(path httprequest.PathItem) {
		path.Tag("Sessions")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.SessionListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := reg.ListSessions(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Sessions")
			op.Description(documentation.Section(3, "GET /llm/session").Body)
			op.Query(jsonschema.MustFor[schema.SessionListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.SessionList](), "List of sessions, without their messages")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var meta schema.SessionMeta
			if err := httprequest.Read(r, &meta); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if session, err := reg.CreateSession(r.Context(), meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Create Session")
			op.Description(documentation.Section(3, "POST /llm/session").Body)
			op.RequestBody(jsonschema.MustFor[schema.SessionMeta]())
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.Session](), "Created session")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, an invalid schema, or an option which is a field")
			op.ErrorResponse(http.StatusNotFound, "Model or tool not found")
			op.ErrorResponse(http.StatusConflict, "The name of the model, or a tool, is ambiguous")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})
	})
}

func sessionGetUpdateDeleteHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/session/{id}", jsonschema.MustFor[sessionID](), func(path httprequest.PathItem) {
		path.Tag("Sessions")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if session, err := reg.GetSession(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Session")
			op.Description(documentation.Section(3, "GET /llm/session/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "Session, with its messages")
			op.ErrorResponse(http.StatusBadRequest, "The identifier isn't a UUID")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})

		// PATCH
		path.Patch(func(w http.ResponseWriter, r *http.Request) {
			var meta schema.SessionMeta
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if err := httprequest.Read(r, &meta); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if session, err := reg.UpdateSession(r.Context(), id, meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Update Session")
			op.Description(documentation.Section(3, "PATCH /llm/session/{id}").Body)
			op.RequestBody(jsonschema.MustFor[schema.SessionMeta]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "Updated session")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, an identifier which isn't a UUID, an update which changes nothing, or settings which, as changed, aren't valid")
			op.ErrorResponse(http.StatusNotFound, "Session, model or tool not found")
			op.ErrorResponse(http.StatusConflict, "The session was changed as it was updated, or the name of the model, or a tool, is ambiguous")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			id, err := pathSessionID(r)
			if err != nil {
				httpresponse.Error(w, err)
			} else if session, err := reg.DeleteSession(r.Context(), id); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), id.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), session)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Session")
			op.Description(documentation.Section(3, "DELETE /llm/session/{id}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Session](), "Deleted session")
			op.ErrorResponse(http.StatusBadRequest, "The identifier isn't a UUID")
			op.ErrorResponse(http.StatusNotFound, "Session not found")
			op.ErrorResponse(http.StatusConflict, "The session was changed as it was deleted")
			op.ErrorResponse(http.StatusServiceUnavailable, "The server has no session store")
		})
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// pathSessionID returns the session's identifier in the request's path, or
// ErrBadRequest when it isn't a UUID
func pathSessionID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.UUID{}, httpresponse.ErrBadRequest.Withf("invalid session identifier %q", r.PathValue("id"))
	}
	return id, nil
}
