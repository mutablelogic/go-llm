package httphandler

import (
	"net/http"

	// Packages
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: /tool
func ToolListHandler(manager *agent.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
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
				Description: "List all tools",
			},
		})
}

// Path: /tool/{name}
func ToolGetHandler(manager *agent.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/tool/{name}", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				resp, err := manager.GetTool(r.Context(), r.PathValue("name"))
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
				Description: "Get a tool by name",
			},
		})
}
