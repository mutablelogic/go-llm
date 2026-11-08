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
// TYPES

// connectorName is the path parameter naming a connector
type connectorName struct {
	Name string `json:"name" help:"Connector name"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func connectorCreateListHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("mcp/connector", nil, func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			if list, err := reg.ListConnectors(r.Context()); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Connectors")
			op.Description(documentation.Section(3, "GET /mcp/connector").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ConnectorList](), "List of connectors")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var create schema.Connector
			if err := httprequest.Read(r, &create); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if connector, err := reg.CreateConnector(r.Context(), create); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), create.Name)
			} else {
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), connector)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Create Connector")
			op.Description(documentation.Section(3, "POST /mcp/connector").Body)
			op.RequestBody(jsonschema.MustFor[schema.Connector]())
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.Connector](), "Created connector")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, name or URL")
			op.ErrorResponse(http.StatusConflict, "A connector with that name exists already")
		})
	})
}

func connectorHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("mcp/connector/{name}", jsonschema.MustFor[connectorName](), func(path httprequest.PathItem) {
		path.Tag(Tag)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if connector, err := reg.GetConnector(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), connector)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Connector")
			op.Description(documentation.Section(3, "GET /mcp/connector/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Connector](), "Connector")
			op.ErrorResponse(http.StatusNotFound, "Connector not found")
		})

		// PATCH
		path.Patch(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			var meta schema.ConnectorMeta
			if err := httprequest.Read(r, &meta); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if connector, err := reg.UpdateConnector(r.Context(), name, meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), connector)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Update Connector")
			op.Description(documentation.Section(3, "PATCH /mcp/connector/{name}").Body)
			op.RequestBody(jsonschema.MustFor[schema.ConnectorMeta]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Connector](), "Updated connector")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, or URL")
			op.ErrorResponse(http.StatusNotFound, "Connector not found")
			op.ErrorResponse(http.StatusConflict, "The connector was changed as it was updated")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if connector, err := reg.DeleteConnector(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), connector)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Connector")
			op.Description(documentation.Section(3, "DELETE /mcp/connector/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Connector](), "Deleted connector")
			op.ErrorResponse(http.StatusNotFound, "Connector not found")
			op.ErrorResponse(http.StatusConflict, "The connector was changed as it was deleted")
		})
	})
}
