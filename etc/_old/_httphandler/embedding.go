package httphandler

import (
	"net/http"

	// Packages
	manager "github.com/mutablelogic/go-llm/kernel/manager"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: embedding
func EmbeddingHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	reqSchema, _ := jsonschema.For[schema.EmbeddingRequest]()
	respSchema, _ := jsonschema.For[schema.EmbeddingResponse]()
	return "embedding", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				var req schema.EmbeddingRequest
				if err := httprequest.Read(r, &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}

				// Call the embedding API
				resp, err := manager.Embedding(r.Context(), &req)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Post: &openapi.Operation{
				Tags:        []string{"Embedding"},
				Description: "Generate embeddings for text input",
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: reqSchema}},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Embedding vectors", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: respSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}
