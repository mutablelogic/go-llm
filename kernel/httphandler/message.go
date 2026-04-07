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

func SessionMessageHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "session/{session}/message", jsonschema.MustFor[schema.SessionIDSelector](), httprequest.NewPathItem(
		"Session message operations",
		"List messages for a session",
		"Sessions",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = listMessages(r.Context(), manager, w, r)
		},
		"List session messages",
		opts.WithQuery(jsonschema.MustFor[schema.MessageListRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.MessageList]()),
		opts.WithErrorResponse(400, "Invalid request parameters or session ID."),
		opts.WithErrorResponse(404, "Session not found."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func listMessages(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	session, err := uuid.Parse(r.PathValue("session"))
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	var req schema.MessageListRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	messages, err := manager.ListMessages(ctx, session, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), messages)
}
