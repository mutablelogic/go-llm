package httphandler

import (
	"net/http"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/mcp/registry"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func templateHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("mcp/template", nil, func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			if list, err := reg.ListTemplates(r.Context()); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Connector Templates")
			op.Description(documentation.Section(3, "GET /mcp/template").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ConnectorTemplateList](), "List of connector templates")
		})
	})
}
