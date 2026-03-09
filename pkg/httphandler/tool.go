package httphandler

import (
	"net/http"

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

// Path: /tool
func ToolListHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	listRespSchema, _ := jsonschema.For[schema.ListToolResponse]()
	return "/tool", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				var req schema.ListToolRequest
				if err := httprequest.Query(r.URL.Query(), &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.ListTools(r.Context(), req)
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
				Tags:        []string{"Tool"},
				Description: "List all tools",
				Parameters: []openapi.Parameter{
					{Name: "limit", In: openapi.ParameterInQuery, Description: "Maximum number of tools to return", Schema: queryUintSchema},
					{Name: "offset", In: openapi.ParameterInQuery, Description: "Offset for pagination", Schema: queryUintSchema},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "List of tools", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: listRespSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}

// Path: /tool/{name}
func ToolGetHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	nameParam := openapi.Parameter{
		Name:        "name",
		In:          openapi.ParameterInPath,
		Description: "Tool name",
		Required:    true,
		Schema:      pathParamSchema,
	}
	toolSchema, _ := jsonschema.For[schema.ToolMeta]()
	return "/tool/{name}", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				resp, err := manager.GetTool(r.Context(), r.PathValue("name"))
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			case http.MethodPost:
				var req schema.CallToolRequest
				if err := httprequest.Read(r, &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.CallTool(r.Context(), r.PathValue("name"), req.Input)
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
				Tags:        []string{"Tool"},
				Description: "Get a tool by name",
				Parameters:  []openapi.Parameter{nameParam},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Tool details", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: toolSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Post: &openapi.Operation{
				Tags:        []string{"Tool"},
				Description: "Call a tool by name",
				Parameters:  []openapi.Parameter{nameParam},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Tool result"},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}
