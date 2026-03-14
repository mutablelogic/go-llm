package httphandler

import (
	"net/http"
	"strings"

	// Packages
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: model
func ModelListHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	listRespSchema, _ := jsonschema.For[schema.ListModelsResponse]()
	downloadReqSchema, _ := jsonschema.For[schema.DownloadModelRequest]()
	modelSchema, _ := jsonschema.For[schema.Model]()
	return "model", func(w http.ResponseWriter, r *http.Request) {
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
			case http.MethodPost:
				var req schema.DownloadModelRequest
				if err := httprequest.Read(r, &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				switch acceptType(r) {
				case acceptStream:
					downloadStream(w, r, manager, req)
				default:
					resp, err := manager.DownloadModel(r.Context(), req)
					if err != nil {
						_ = httpresponse.Error(w, httpErr(err))
						return
					}
					_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
				}
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
			Post: &openapi.Operation{
				Tags:        []string{"Model"},
				Description: "Download a model",
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: downloadReqSchema}},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Downloaded model details", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: modelSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}

// Path: model/{model...}
func ModelGetHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	modelParam := openapi.Parameter{
		Name:        "model",
		In:          openapi.ParameterInPath,
		Description: "Provider and model name (e.g. \"anthropic/claude-3\")",
		Required:    true,
		Schema:      pathParamSchema,
	}
	modelSchema, _ := jsonschema.For[schema.Model]()
	return "model/{model...}", func(w http.ResponseWriter, r *http.Request) {
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
			case http.MethodDelete:
				// Set model name and provider from path parameter
				var req schema.DeleteModelRequest
				if len(provider_model) == 2 {
					req.Provider = provider_model[0]
					req.Name = provider_model[1]
				} else {
					req.Name = provider_model[0]
				}

				// Delete the model via the manager
				if err := manager.DeleteModel(r.Context(), req); err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				w.WriteHeader(http.StatusNoContent)
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
			Delete: &openapi.Operation{
				Tags:        []string{"Model"},
				Description: "Delete a model",
				Parameters:  []openapi.Parameter{modelParam},
				Responses: map[string]openapi.Response{
					"204":     {Description: "Model deleted successfully"},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}

// downloadStream streams model download progress as SSE events, then emits a
// final "result" event containing the downloaded model.
func downloadStream(w http.ResponseWriter, r *http.Request, m *manager.Manager, req schema.DownloadModelRequest) {
	stream := httpresponse.NewTextStream(w)
	if stream == nil {
		_ = httpresponse.Error(w, httpresponse.ErrInternalError)
		return
	}
	defer stream.Close()

	progressFn := opt.ProgressFn(func(status string, percent float64) {
		stream.Write(schema.EventProgress, schema.ProgressEvent{Status: status, Percent: percent})
	})

	model, err := m.DownloadModel(r.Context(), req, opt.WithProgress(progressFn))
	if err != nil {
		stream.Write(schema.EventError, schema.StreamError{Error: err.Error()})
		return
	}
	stream.Write(schema.EventResult, model)
}
