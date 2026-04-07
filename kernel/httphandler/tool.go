package httphandler

import (
	"context"
	"io"
	"net/http"
	"net/url"

	// Packages
	middleware "github.com/djthorpe/go-auth/pkg/middleware"
	llm "github.com/mutablelogic/go-llm"
	llmmanager "github.com/mutablelogic/go-llm/kernel/manager"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func ToolHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "tool", nil, httprequest.NewPathItem(
		"Tool operations",
		"List operations on tools",
		"Tools & Agents",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = listTools(r.Context(), manager, w, r)
		},
		"List tools",
		opts.WithQuery(jsonschema.MustFor[schema.ToolListRequest]()),
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.ToolList]()),
		opts.WithErrorResponse(400, "Invalid request parameters or tool listing failure."),
	)
}

func ToolResourceHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "tool/{name}", nil, httprequest.NewPathItem(
		"Tool operations",
		"Get and call operations on tools",
		"Tools & Agents",
	).Get(
		func(w http.ResponseWriter, r *http.Request) {
			_ = getTool(r.Context(), manager, w, r)
		},
		"Get tool",
		opts.WithJSONResponse(200, jsonschema.MustFor[schema.ToolMeta]()),
		opts.WithErrorResponse(404, "Tool not found."),
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			_ = callTool(r.Context(), manager, w, r)
		},
		"Call tool",
		opts.WithJSONRequest(jsonschema.MustFor[schema.CallToolRequest]()),
		opts.WithResponse(200, types.ContentTypeJSON, jsonschema.MustFor[map[string]any](), "Tool result returned as raw resource content. Actual content type may vary by tool."),
		opts.WithResponse(200, types.ContentTypeTextPlain, jsonschema.MustFor[string](), "Tool result returned as raw text content. Actual content type may vary by tool."),
		opts.WithNoContentResponse(204, "Tool returned no content."),
		opts.WithErrorResponse(400, "Invalid request body or tool call failure."),
		opts.WithErrorResponse(404, "Tool not found."),
		opts.WithErrorResponse(409, "Multiple tools matched; specify a fully-qualified tool name."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func listTools(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	var req schema.ToolListRequest
	if err := httprequest.Query(r.URL.Query(), &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	tools, err := manager.ListTools(ctx, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), tools)
}

func getTool(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	name, err := unescapePathValue(r, "name")
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	tool, err := manager.GetTool(ctx, name, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), tool)
}

func callTool(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	name, err := unescapePathValue(r, "name")
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	var req schema.CallToolRequest
	if err := httprequest.Read(r, &req); err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}

	resource, err := manager.CallTool(ctx, name, req, middleware.UserFromContext(ctx))
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	return writeToolResource(ctx, w, resource)
}

func writeToolResource(ctx context.Context, w http.ResponseWriter, resource llm.Resource) error {
	if resource == nil {
		return httpresponse.Write(w, http.StatusNoContent, types.ContentTypeTextPlain, nil)
	}

	data, err := resource.Read(ctx)
	if err != nil {
		return httpresponse.Error(w, schema.HTTPErr(schema.ErrInternalServerError.Withf("reading tool result: %v", err)))
	}

	contentType := resource.Type()
	if contentType == "" {
		contentType = types.ContentTypeTextPlain
	}
	if uri := resource.URI(); uri != "" {
		w.Header().Set(types.ContentPathHeader, uri)
	}
	if name := resource.Name(); name != "" {
		w.Header().Set(types.ContentNameHeader, name)
	}
	if description := resource.Description(); description != "" {
		w.Header().Set(types.ContentDescriptionHeader, description)
	}

	return httpresponse.Write(w, http.StatusOK, contentType, func(writer io.Writer) (int, error) {
		return writer.Write(data)
	})
}

func unescapePathValue(r *http.Request, key string) (string, error) {
	return url.PathUnescape(r.PathValue(key))
}
