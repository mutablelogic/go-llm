package httphandler

import (
	"context"
	"net/http"

	// Packages
	middleware "github.com/djthorpe/go-auth/pkg/middleware"
	llmmanager "github.com/mutablelogic/go-llm/pkg/llmmanager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func AgentHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "agent", nil, httprequest.NewPathItem(
		"Agent operations",
		"List externally exposed agents",
		"Tools & Agents",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = listAgents(r.Context(), manager, w, r)
		},
		"List agents",
		opts.WithQuery(jsonschema.MustFor[schema.AgentListRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.AgentList]()),
		opts.WithErrorResponse(400, "Invalid request parameters or agent listing failure."),
	)
}

func AgentResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "agent/{name}", nil, httprequest.NewPathItem(
		"Agent operations",
		"Get operations on agents",
		"Tools & Agents",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getAgent(r.Context(), manager, w, r)
		},
		"Get agent",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.AgentMeta]()),
		opts.WithErrorResponse(400, "Invalid agent path parameter."),
		opts.WithErrorResponse(404, "Agent not found."),
		opts.WithErrorResponse(409, "Multiple agents matched; specify a fully-qualified agent name."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func listAgents(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.AgentListRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	agents, err := manager.ListAgents(ctx, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), agents)
}

func getAgent(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	name, err := unescapePathValue(r, "name")
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	agent, err := manager.GetAgent(ctx, name, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), agent)
}
