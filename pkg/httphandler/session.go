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

// Path: /session
func SessionHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	sessionMetaSchema, _ := jsonschema.For[schema.SessionMeta]()
	listRespSchema, _ := jsonschema.For[schema.ListSessionResponse]()
	sessionSchema, _ := jsonschema.For[schema.Session]()
	return "/session", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				var req schema.ListSessionRequest
				if err := httprequest.Query(r.URL.Query(), &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.ListSessions(r.Context(), req)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			case http.MethodPost:
				var req schema.SessionMeta
				if err := httprequest.Read(r, &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.CreateSession(r.Context(), req)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), resp)
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Get: &openapi.Operation{
				Tags:        []string{"Session"},
				Description: "List all sessions",
				Parameters: []openapi.Parameter{
					{Name: "limit", In: openapi.ParameterInQuery, Description: "Maximum number of sessions to return", Schema: queryUintSchema},
					{Name: "offset", In: openapi.ParameterInQuery, Description: "Offset for pagination", Schema: queryUintSchema},
					{Name: "label", In: openapi.ParameterInQuery, Description: "Filter by labels (key:value)", Schema: queryStringArraySchema},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "List of sessions", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: listRespSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Post: &openapi.Operation{
				Tags:        []string{"Session"},
				Description: "Create a new session",
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: sessionMetaSchema}},
				},
				Responses: map[string]openapi.Response{
					"201":     {Description: "Session created", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: sessionSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}

// Path: /session/{session}
func SessionGetHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	sessionParam := openapi.Parameter{
		Name:        "session",
		In:          openapi.ParameterInPath,
		Description: "Session ID",
		Required:    true,
		Schema:      pathParamSchema,
	}
	sessionMetaSchema, _ := jsonschema.For[schema.SessionMeta]()
	sessionSchema, _ := jsonschema.For[schema.Session]()
	return "/session/{session}", func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("session")
			switch r.Method {
			case http.MethodGet:
				resp, err := manager.GetSession(r.Context(), id)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			case http.MethodDelete:
				if _, err := manager.DeleteSession(r.Context(), id); err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			case http.MethodPatch:
				var req schema.SessionMeta
				if err := httprequest.Read(r, &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.UpdateSession(r.Context(), id, req)
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
				Tags:        []string{"Session"},
				Description: "Get a session by ID",
				Parameters:  []openapi.Parameter{sessionParam},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Session details", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: sessionSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Delete: &openapi.Operation{
				Tags:        []string{"Session"},
				Description: "Delete a session by ID",
				Parameters:  []openapi.Parameter{sessionParam},
				Responses: map[string]openapi.Response{
					"204":     {Description: "Session deleted"},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Patch: &openapi.Operation{
				Tags:        []string{"Session"},
				Description: "Update a session's metadata",
				Parameters:  []openapi.Parameter{sessionParam},
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: sessionMetaSchema}},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Updated session", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: sessionSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}
