package httphandler

import (
	"net/http"
	"net/url"

	// Packages
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: /connector
func ConnectorListHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	listRespSchema, _ := jsonschema.For[schema.ListConnectorsResponse]()
	return "/connector", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				var req schema.ListConnectorsRequest
				if err := httprequest.Query(r.URL.Query(), &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.ListConnectors(r.Context(), req)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Get: &openapi.Operation{
				Tags:        []string{"Connector"},
				Description: "List registered MCP server connectors",
				Parameters: []openapi.Parameter{
					{Name: "namespace", In: openapi.ParameterInQuery, Description: "Filter by namespace", Schema: pathParamSchema},
					{Name: "enabled", In: openapi.ParameterInQuery, Description: "Filter by enabled state", Schema: queryBoolSchema},
					{Name: "limit", In: openapi.ParameterInQuery, Description: "Maximum number of connectors to return", Schema: queryUintSchema},
					{Name: "offset", In: openapi.ParameterInQuery, Description: "Offset for pagination", Schema: queryUintSchema},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "List of connectors", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: listRespSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}

// Path: /connector/{url}
func ConnectorHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	urlParam := openapi.Parameter{
		Name:        "url",
		In:          openapi.ParameterInPath,
		Description: "MCP server URL",
		Required:    true,
		Schema:      pathParamSchema,
	}
	connectorMetaSchema, _ := jsonschema.For[schema.ConnectorMeta]()
	connectorSchema, _ := jsonschema.For[schema.Connector]()
	return "/connector/{url}", func(w http.ResponseWriter, r *http.Request) {
			rawURL, err := url.PathUnescape(r.PathValue("url"))
			if err != nil {
				_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With(err))
				return
			}
			switch r.Method {
			case http.MethodGet:
				resp, err := manager.GetConnector(r.Context(), rawURL)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			case http.MethodPost:
				var meta schema.ConnectorMeta
				if err := httprequest.Read(r, &meta); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.CreateConnector(r.Context(), rawURL, meta)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), resp)
			case http.MethodPatch:
				var meta schema.ConnectorMeta
				if err := httprequest.Read(r, &meta); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.UpdateConnector(r.Context(), rawURL, meta)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			case http.MethodDelete:
				if err := manager.DeleteConnector(r.Context(), rawURL); err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Get: &openapi.Operation{
				Tags:        []string{"Connector"},
				Description: "Get a registered MCP server connector by URL",
				Parameters:  []openapi.Parameter{urlParam},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Connector details", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: connectorSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Post: &openapi.Operation{
				Tags:        []string{"Connector"},
				Description: "Register a new MCP server connector",
				Parameters:  []openapi.Parameter{urlParam},
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: connectorMetaSchema}},
				},
				Responses: map[string]openapi.Response{
					"201":     {Description: "Connector registered", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: connectorSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Patch: &openapi.Operation{
				Tags:        []string{"Connector"},
				Description: "Update the metadata for a registered MCP server connector",
				Parameters:  []openapi.Parameter{urlParam},
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: connectorMetaSchema}},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Updated connector", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: connectorSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Delete: &openapi.Operation{
				Tags:        []string{"Connector"},
				Description: "Delete a registered MCP server connector",
				Parameters:  []openapi.Parameter{urlParam},
				Responses: map[string]openapi.Response{
					"204":     {Description: "Connector deleted"},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}
