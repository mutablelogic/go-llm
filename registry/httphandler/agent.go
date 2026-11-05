package httphandler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// agentName is the path parameter of an agent's name
type agentName struct {
	Name string `json:"name" help:"Name of the agent"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// agentBodyLimit is the largest definition of an agent, which is read into
// memory
const agentBodyLimit = 1 << 20

// yamlContentTypes are the content types of a definition in YAML
var yamlContentTypes = []string{types.ContentTypeYAML, "application/x-yaml", "text/yaml", "text/x-yaml"}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func agentHandlers(r *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return errors.Join(
		agentCreateHandlers(r, router, documentation),
		agentGetHandlers(r, router, documentation),
	)
}

func agentCreateHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/agent", nil, func(path httprequest.PathItem) {
		path.Tag(taggroup.ToolsAndAgents)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.AgentListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := reg.ListAgents(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Agents")
			op.Description(documentation.Section(3, "GET /llm/agent").Body)
			op.Query(jsonschema.MustFor[schema.AgentListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.AgentList](), "List of agents, in the store's order")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			// It's created, or, when there's an agent with its name, the agent
			// is replaced with it, when it's changed
			if meta, err := readAgentMeta(w, r); err != nil {
				httpresponse.Error(w, err)
			} else if agent, created, err := reg.CreateAgent(r.Context(), meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), meta.Name)
			} else if created {
				httpresponse.JSON(w, http.StatusCreated, httprequest.Indent(r), agent)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), agent)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Create or Replace Agent")
			op.Description(documentation.Section(3, "POST /llm/agent").Body)
			op.RequestBody(jsonschema.MustFor[schema.AgentMeta](), append([]string{types.ContentTypeJSON}, yamlContentTypes...)...)
			op.JSONResponse(http.StatusCreated, jsonschema.MustFor[schema.Agent](), "Created agent")
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Agent](), "The agent with the name, replaced, when it's changed, or as it is")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, such as a name which isn't valid, an input which isn't a JSON schema of an object, a prompt which isn't a template, an invalid schema, or an option which is a field")
			op.ErrorResponse(http.StatusNotFound, "Model or tool not found")
			op.ErrorResponse(http.StatusConflict, "The agent was changed as it was replaced, or the name of the model, or a tool, is ambiguous")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 1 MiB")
		})
	})
}

func agentGetHandlers(reg *registry.Registry, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/agent/{name}", jsonschema.MustFor[agentName](), func(path httprequest.PathItem) {
		path.Tag(taggroup.ToolsAndAgents)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if accept, err := acceptAgent(r); err != nil {
				httpresponse.Error(w, err)
			} else if agent, err := reg.GetAgent(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else if accept == types.ContentTypeYAML {
				writeAgentYAML(w, agent.AgentMeta)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), agent)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Agent")
			op.Description(documentation.Section(3, "GET /llm/agent/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Agent](), "Agent, as JSON, or its definition, as YAML, when YAML is accepted")
			op.Response(http.StatusOK, types.ContentTypeYAML, "The agent's definition, as YAML")
			op.ErrorResponse(http.StatusNotFound, "Agent not found")
			op.ErrorResponse(http.StatusNotAcceptable, "The response can't be any of the accepted types")
		})

		// PATCH
		path.Patch(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if accept, err := acceptAgent(r); err != nil {
				httpresponse.Error(w, err)
			} else if meta, err := readAgentMeta(w, r); err != nil {
				httpresponse.Error(w, err)
			} else if agent, err := reg.UpdateAgent(r.Context(), name, meta); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else if accept == types.ContentTypeYAML {
				writeAgentYAML(w, agent.AgentMeta)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), agent)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Update Agent")
			op.Description(documentation.Section(3, "PATCH /llm/agent/{name}").Body)
			op.RequestBody(agentUpdateSchema(), append([]string{types.ContentTypeJSON}, yamlContentTypes...)...)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Agent](), "Updated agent, as JSON, or its definition, as YAML, when YAML is accepted")
			op.Response(http.StatusOK, types.ContentTypeYAML, "The updated agent's definition, as YAML")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request, an update which changes nothing, or a definition which, as changed, isn't valid")
			op.ErrorResponse(http.StatusNotFound, "Agent, model or tool not found")
			op.ErrorResponse(http.StatusNotAcceptable, "The response can't be any of the accepted types")
			op.ErrorResponse(http.StatusConflict, "The agent was changed as it was updated, or the name of the model, or a tool, is ambiguous")
			op.ErrorResponse(http.StatusRequestEntityTooLarge, "The request body is larger than 1 MiB")
		})

		// DELETE
		path.Delete(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if agent, err := reg.DeleteAgent(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), agent)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Delete Agent")
			op.Description(documentation.Section(3, "DELETE /llm/agent/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Agent](), "Deleted agent")
			op.ErrorResponse(http.StatusNotFound, "Agent not found")
			op.ErrorResponse(http.StatusConflict, "The agent was changed as it was deleted")
		})
	})
}

// agentUpdateSchema returns the schema of an update of an agent, which is
// its definition, without its name, which the path has, so nothing is
// required
func agentUpdateSchema() *jsonschema.Schema {
	// The schema is made again from JSON, as it's validated as it was made
	update := *jsonschema.MustFor[schema.AgentMeta]()
	update.Required = slices.DeleteFunc(slices.Clone(update.Required), func(field string) bool { return field == "name" })
	data, err := json.Marshal(&update)
	if err != nil {
		panic(err)
	}
	result, err := jsonschema.FromJSON(data)
	if err != nil {
		panic(err)
	}
	return result
}

// acceptAgent returns the content type which an agent is returned as, which
// is JSON, unless YAML is accepted, or returns 406 when the request accepts
// neither
func acceptAgent(r *http.Request) (string, error) {
	accept, err := types.AcceptContentType(r)
	if err != nil {
		return "", httpresponse.ErrBadRequest.With(err.Error())
	}
	switch {
	case accept == "" || accept == types.ContentTypeAny || accept == types.ContentTypeJSON:
		return types.ContentTypeJSON, nil
	case slices.Contains(yamlContentTypes, accept):
		return types.ContentTypeYAML, nil
	default:
		return "", httpresponse.Err(http.StatusNotAcceptable).Withf("cannot respond with %q", accept)
	}
}

// writeAgentYAML writes the agent's definition, as YAML, as a file of it
// is, which can be created again, as it is
func writeAgentYAML(w http.ResponseWriter, meta schema.AgentMeta) {
	data, err := meta.YAML()
	if err != nil {
		httpresponse.Error(w, httpresponse.ErrInternalError.With(err.Error()))
		return
	}
	w.Header().Set(types.ContentTypeHeader, types.ContentTypeYAML)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// readAgentMeta returns the definition of an agent in the request's body,
// which is JSON, or YAML, such as a file of an agent's definition, by the
// names its JSON has, and at most agentBodyLimit bytes
func readAgentMeta(w http.ResponseWriter, r *http.Request) (schema.AgentMeta, error) {
	contentType, err := types.RequestContentType(r)
	if err != nil {
		return schema.AgentMeta{}, httpresponse.ErrBadRequest.With(err.Error())
	}
	if err := limitBody(w, r, agentBodyLimit); err != nil {
		return schema.AgentMeta{}, err
	}
	for _, yamlType := range yamlContentTypes {
		if contentType == yamlType {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				return schema.AgentMeta{}, httpresponse.ErrBadRequest.With(err.Error())
			}
			meta, err := schema.ParseAgentMeta(data)
			if err != nil {
				return schema.AgentMeta{}, llm.HTTPErr(err)
			}
			return meta, nil
		}
	}
	var meta schema.AgentMeta
	if err := httprequest.Read(r, &meta); err != nil {
		return schema.AgentMeta{}, httpresponse.ErrBadRequest.Withf("%w", err)
	}
	return meta, nil
}
