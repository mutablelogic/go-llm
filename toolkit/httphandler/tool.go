package httphandler

import (
	"encoding/json"
	"errors"
	"net/http"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// toolName is the path parameter naming a tool
type toolName struct {
	Name string `json:"name" help:"Canonical name of the tool, such as weatherapi:current, or its name without a namespace"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func toolHandlers(tools *toolkit.Toolkit, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag("Tools", documentation.Section(2, "Tool Operations").Body)
	return errors.Join(
		toolListHandlers(tools, router, documentation),
		toolGetCallHandlers(tools, router, documentation),
	)
}

func toolListHandlers(tools *toolkit.Toolkit, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/tool", nil, func(path httprequest.PathItem) {
		path.Tag("Tools")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.ToolListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := tools.ListTools(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Tools")
			op.Description(documentation.Section(3, "GET /llm/tool").Body)
			op.Query(jsonschema.MustFor[schema.ToolListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ToolList](), "List of tools")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
		})
	})
}

func toolGetCallHandlers(tools *toolkit.Toolkit, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/tool/{name}", jsonschema.MustFor[toolName](), func(path httprequest.PathItem) {
		path.Tag("Tools")

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if tool, err := tools.GetTool(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), tool)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Tool")
			op.Description(documentation.Section(3, "GET /llm/tool/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Tool](), "Tool")
			op.ErrorResponse(http.StatusNotFound, "Tool not found")
			op.ErrorResponse(http.StatusConflict, "The name has no namespace, and more than one namespace has a tool with the name")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var req schema.CallToolRequest
			name := r.PathValue("name")
			if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if result, err := tools.CallTool(r.Context(), name, req.Input); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else if result == nil {
				httpresponse.Empty(w, http.StatusNoContent)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), result)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Call Tool")
			op.Description(documentation.Section(3, "POST /llm/tool/{name}").Body)
			op.RequestBody(jsonschema.MustFor[schema.CallToolRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[json.RawMessage](), "Result of the tool as JSON")
			op.EmptyResponse("The tool has no result")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body, or the input doesn't match the tool's input schema")
			op.ErrorResponse(http.StatusNotFound, "Tool not found")
			op.ErrorResponse(http.StatusConflict, "The name has no namespace, and more than one namespace has a tool with the name")
		})
	})
}
