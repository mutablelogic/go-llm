package httphandler

import (
	"net/http"

	// Packages
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: /session
func SessionHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
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
			},
			Post: &openapi.Operation{
				Tags:        []string{"Session"},
				Description: "Create a new session",
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
			},
			Delete: &openapi.Operation{
				Tags:        []string{"Session"},
				Description: "Delete a session by ID",
				Parameters:  []openapi.Parameter{sessionParam},
			},
			Patch: &openapi.Operation{
				Tags:        []string{"Session"},
				Description: "Update a session's metadata",
				Parameters:  []openapi.Parameter{sessionParam},
			},
		})
}
