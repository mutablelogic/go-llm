package httphandler

import (
	"net/http"
	"strings"

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

// Path: /model
func ModelListHandler(manager *agent.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/model", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				var req schema.ListModelsRequest
				if err := httprequest.Query(r.URL.Query(), &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.ListModels(r.Context(), req)
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
				Description: "List all models",
			},
		})
}

// Path: /model/{model...}
func ModelGetHandler(manager *agent.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/model/{model...}", func(w http.ResponseWriter, r *http.Request) {
			provider_model := strings.SplitN(r.PathValue("model"), PathSeparator, 2)
			switch r.Method {
			case http.MethodGet:
				// Set model name and provider from path parameter
				var req schema.GetModelRequest
				if len(provider_model) == 2 {
					req.Provider = provider_model[0]
					req.Name = provider_model[1]
				} else {
					req.Name = provider_model[0]
				}

				// Get the model from the manager
				resp, err := manager.GetModel(r.Context(), req)
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
				Description: "Get a model by ID",
			},
		})
}
