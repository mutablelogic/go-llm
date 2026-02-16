package httphandler

import (
	"mime"
	"net/http"
	"strings"

	// Packages
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: /agent
func AgentHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/agent", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				var req schema.ListAgentRequest
				if err := httprequest.Query(r.URL.Query(), &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				if req.Version != nil && req.Name == "" {
					_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With("version requires name"))
					return
				}
				resp, err := manager.ListAgents(r.Context(), req)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			case http.MethodPost:
				req, err := readAgentMeta(r)
				if err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.CreateAgent(r.Context(), req)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), resp)
			case http.MethodPut:
				req, err := readAgentMeta(r)
				if err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				if req.Name == "" {
					_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With("name is required for update"))
					return
				}
				existing, err := manager.GetAgent(r.Context(), req.Name)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				resp, err := manager.UpdateAgent(r.Context(), existing.ID, req)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				if resp.Version == existing.Version {
					w.WriteHeader(http.StatusNotModified)
				} else {
					_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
				}
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Get: &openapi.Operation{
				Description: "List all agents",
			},
			Post: &openapi.Operation{
				Description: "Create a new agent",
			},
			Put: &openapi.Operation{
				Description: "Update an existing agent by name",
			},
		})
}

// Path: /agent/{agent}
func AgentGetHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/agent/{agent}", func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("agent")
			switch r.Method {
			case http.MethodGet:
				resp, err := manager.GetAgent(r.Context(), id)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			case http.MethodPost:
				var req schema.CreateAgentSessionRequest
				if err := httprequest.Read(r, &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				resp, err := manager.CreateAgentSession(r.Context(), id, req)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), resp)
			case http.MethodDelete:
				if _, err := manager.DeleteAgent(r.Context(), id); err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Get: &openapi.Operation{
				Description: "Get an agent by ID or name",
			},
			Post: &openapi.Operation{
				Description: "Create a session from an agent definition",
			},
			Delete: &openapi.Operation{
				Description: "Delete an agent by ID or name",
			},
		})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// readAgentMeta reads an AgentMeta from the request body.
// Supports application/json (default), text/plain, and text/markdown content types.
func readAgentMeta(r *http.Request) (schema.AgentMeta, error) {
	ct := r.Header.Get("Content-Type")
	if ct != "" {
		mediaType, _, _ := mime.ParseMediaType(ct)
		if strings.HasPrefix(mediaType, "text/") {
			meta, err := agent.Read(r.Body)
			if err != nil {
				return schema.AgentMeta{}, httpresponse.ErrBadRequest.With(err)
			}
			return meta, nil
		}
	}

	// Default: JSON
	var req schema.AgentMeta
	if err := httprequest.Read(r, &req); err != nil {
		return schema.AgentMeta{}, err
	}
	return req, nil
}
