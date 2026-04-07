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
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: agent
func AgentHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	agentMetaSchema, _ := jsonschema.For[schema.AgentMeta]()
	listRespSchema, _ := jsonschema.For[schema.ListAgentResponse]()
	agentSchema, _ := jsonschema.For[schema.Agent]()
	return "agent", func(w http.ResponseWriter, r *http.Request) {
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
				Tags:        []string{"Agent"},
				Description: "List all agents",
				Parameters: []openapi.Parameter{
					{Name: "name", In: openapi.ParameterInQuery, Description: "Filter by agent name", Schema: pathParamSchema},
					{Name: "version", In: openapi.ParameterInQuery, Description: "Filter by version number (requires name)", Schema: queryUintSchema},
					{Name: "limit", In: openapi.ParameterInQuery, Description: "Maximum number of agents to return", Schema: queryUintSchema},
					{Name: "offset", In: openapi.ParameterInQuery, Description: "Offset for pagination", Schema: queryUintSchema},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "List of agents", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: listRespSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Post: &openapi.Operation{
				Tags:        []string{"Agent"},
				Description: "Create a new agent",
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: agentMetaSchema}},
				},
				Responses: map[string]openapi.Response{
					"201":     {Description: "Agent created", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: agentSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Put: &openapi.Operation{
				Tags:        []string{"Agent"},
				Description: "Update an existing agent by name",
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: agentMetaSchema}},
				},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Updated agent", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: agentSchema}}},
					"304":     {Description: "Agent unchanged"},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
		})
}

// Path: agent/{agent}
func AgentGetHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	agentParam := openapi.Parameter{
		Name:        "agent",
		In:          openapi.ParameterInPath,
		Description: "Agent ID or name",
		Required:    true,
		Schema:      pathParamSchema,
	}
	createAgentSessionSchema, _ := jsonschema.For[schema.CreateAgentSessionRequest]()
	agentSchema, _ := jsonschema.For[schema.Agent]()
	createAgentSessionRespSchema, _ := jsonschema.For[schema.CreateAgentSessionResponse]()
	return "agent/{agent}", func(w http.ResponseWriter, r *http.Request) {
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
				Tags:        []string{"Agent"},
				Description: "Get an agent by ID or name",
				Parameters:  []openapi.Parameter{agentParam},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Agent details", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: agentSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Post: &openapi.Operation{
				Tags:        []string{"Agent"},
				Description: "Create a session from an agent definition",
				Parameters:  []openapi.Parameter{agentParam},
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: createAgentSessionSchema}},
				},
				Responses: map[string]openapi.Response{
					"201":     {Description: "Session created", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: createAgentSessionRespSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Delete: &openapi.Operation{
				Tags:        []string{"Agent"},
				Description: "Delete an agent by ID or name",
				Parameters:  []openapi.Parameter{agentParam},
				Responses: map[string]openapi.Response{
					"204":     {Description: "Agent deleted"},
					"default": openapi.ErrorResponse("An error occurred"),
				},
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
