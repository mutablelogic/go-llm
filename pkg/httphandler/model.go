package httphandler

import (
	"net/http"
	"strings"

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

// Path: /model
func ModelListHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	listRespSchema, _ := jsonschema.For[schema.ListModelsResponse]()
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
				Tags:        []string{"Model"},
				Description: "List all models",
				Parameters: []openapi.Parameter{
					{Name: "provider", In: openapi.ParameterInQuery, Description: "Filter by provider name", Schema: pathParamSchema},
					{Name: "limit", In: openapi.ParameterInQuery, Description: "Maximum number of models to return", Schema: queryUintSchema},
					{Name: "offset", In: openapi.ParameterInQuery, Description: "Offset for pagination", Schema: queryUintSchema},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "List of models", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: listRespSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}

// Path: /model/{model...}
func ModelGetHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	modelParam := openapi.Parameter{
		Name:        "model",
		In:          openapi.ParameterInPath,
		Description: "Provider and model name (e.g. \"anthropic/claude-3\")",
		Required:    true,
		Schema:      pathParamSchema,
	}
	modelSchema, _ := jsonschema.For[schema.Model]()
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
				Tags:        []string{"Model"},
				Description: "Get a model by ID",
				Parameters:  []openapi.Parameter{modelParam},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Model details", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: modelSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}
