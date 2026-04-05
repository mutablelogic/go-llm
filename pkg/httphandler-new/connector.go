package httphandler

import (
	"context"
	"errors"
	"net/http"
	"net/url"

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

func ConnectorHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "connector", nil, httprequest.NewPathItem(
		"Connector operations",
		"List and create operations on connectors",
		"Connector",
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = createConnector(r.Context(), manager, w, r)
		},
		"Create connector",
		opts.WithDescription("Registers an MCP connector after probing the target server. Returns the created connector on success, or a standard unauthorized error when the connector requires user authorization before registration can complete. In that case, the error detail contains the authorization code flow metadata and requested scopes."),
		opts.WithJSONRequest(jsonschema.MustFor[schema.ConnectorInsert]()),
		opts.WithJSONResponse(201, jsonschema.MustFor[schema.Connector]()),
		opts.WithErrorResponse(401, "Connector authorization is required before registration can complete. The error detail contains a CreateConnectorUnauthorizedResponse with the authorization code flow metadata and requested scopes."),
		opts.WithErrorResponse(400, "Invalid request body or connector creation failure."),
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = listConnectors(r.Context(), manager, w, r)
		},
		"List connectors",
		opts.WithQuery(jsonschema.MustFor[schema.ConnectorListRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.ConnectorList]()),
		opts.WithErrorResponse(400, "Invalid request parameters or connector listing failure."),
	)
}

func ConnectorResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "connector/{url}", jsonschema.MustFor[schema.ConnectorURLSelector](), httprequest.NewPathItem(
		"Connector operations",
		"Get, update, and delete operations on connectors",
		"Connector",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getConnector(r.Context(), manager, w, r)
		},
		"Get connector",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Connector]()),
		opts.WithErrorResponse(400, "Invalid connector URL."),
		opts.WithErrorResponse(404, "Connector not found."),
	).Patch(
		func(w http.ResponseWriter, r *http.Request) {
			_ = updateConnector(r.Context(), manager, w, r)
		},
		"Update connector",
		opts.WithJSONRequest(jsonschema.MustFor[schema.ConnectorMeta]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Connector]()),
		opts.WithErrorResponse(400, "Invalid connector URL, request body, or connector update failure."),
		opts.WithErrorResponse(404, "Connector not found."),
	).Delete(
		func(w http.ResponseWriter, r *http.Request) {
			_ = deleteConnector(r.Context(), manager, w, r)
		},
		"Delete connector",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.Connector]()),
		opts.WithErrorResponse(400, "Invalid connector URL."),
		opts.WithErrorResponse(404, "Connector not found."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func listConnectors(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.ConnectorListRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	connectors, err := manager.ListConnectors(ctx, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), connectors)
}

func createConnector(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.ConnectorInsert
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	connector, codeflow, scopes, err := manager.CreateConnector(ctx, req, middleware.UserFromContext(ctx))
	if errors.Is(err, httpresponse.ErrNotAuthorized) {
		return httpresponse.Error(w, err, schema.CreateConnectorUnauthorizedResponse{
			CodeFlow: codeflow,
			Scopes:   scopes,
		})
	} else if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	// Respond with the connector
	return httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), connector)
}

func getConnector(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	rawURL, err := url.PathUnescape(r.PathValue("url"))
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	connector, err := manager.GetConnector(ctx, rawURL, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), connector)
}

func updateConnector(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	rawURL, err := url.PathUnescape(r.PathValue("url"))
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	var req schema.ConnectorMeta
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	connector, err := manager.UpdateConnector(ctx, rawURL, req)
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), connector)
}

func deleteConnector(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	rawURL, err := url.PathUnescape(r.PathValue("url"))
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	connector, err := manager.DeleteConnector(ctx, rawURL)
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), connector)
}
