package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	registryschema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Agents are the registry's agents, which the agent tool lists, gets,
// creates, and updates, of the agent namespace, which can be changed
type Agents interface {
	ListAgents(ctx context.Context, req registryschema.AgentListRequest) (*registryschema.AgentList, error)
	GetAgent(ctx context.Context, name string) (*registryschema.Agent, error)
	CreateAgent(ctx context.Context, meta registryschema.AgentMeta) (*registryschema.Agent, bool, error)
	UpdateAgent(ctx context.Context, name string, meta registryschema.AgentMeta) (*registryschema.Agent, error)
}

// AgentsFunc returns the agents, or nil, when there are none yet, such as
// when the registry, which has the toolkit, isn't made yet
type AgentsFunc func() Agents

// agentTool lists, gets, creates, and updates the agents of the agent
// namespace, which people, and chats, make, rather than the built-in ones,
// or connectors' prompts, which can't be changed
type agentTool struct {
	toolkit.Base
	agents AgentsFunc
}

var _ gollm.Tool = (*agentTool)(nil)

// agentInput is what the agent tool is asked to do: list the agents, get
// one, create one, or update one, with the fields to set
type agentInput struct {
	Action      string         `json:"action" enum:"list,get,create,update" help:"list the agents which can be changed, get one, by its name, create one, or update one, with the fields to change"`
	Name        string         `json:"name,omitempty" help:"Name of the agent, an identifier, such as summarize_inbox, which it's called by, as a tool, agent:<name>, for get, create and update" optional:""`
	Title       *string        `json:"title,omitempty" help:"Title of the agent, for people" optional:""`
	Description *string        `json:"description,omitempty" help:"What the agent does, for people, and the models which call it as a tool" optional:""`
	Input       map[string]any `json:"input,omitempty" help:"JSON schema of the agent's input, an object, whose properties its prompt uses, such as {\"type\":\"object\",\"properties\":{\"text\":{\"type\":\"string\"}},\"required\":[\"text\"]}" optional:""`
	Prompt      *string        `json:"prompt,omitempty" help:"Go template of the message to the agent's model, executed with its input, such as Summarise this: {{ .text }}" optional:""`
	System      *string        `json:"system,omitempty" help:"System prompt of the agent's model, such as how it acts" optional:""`
	Model       *string        `json:"model,omitempty" help:"Model of the agent, which must call tools, or none, for the model of the conversation which calls it, as llm:models lists them" optional:""`
	Tools       []string       `json:"tools,omitempty" help:"Tools the agent calls, such as weatherapi:*, or none" optional:""`
	Schema      map[string]any `json:"schema,omitempty" help:"JSON schema of the agent's reply, or text when it's omitted" optional:""`
}

// agentSummary is an agent, as it's listed
type agentSummary struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Tool        string `json:"tool"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Model       string `json:"model,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*agentTool) Name() string {
	return "agent"
}

func (*agentTool) Description() string {
	return "Lists, gets, creates and updates agents, each of which is a tool, such as agent:<name>, which asks a model its prompt, with its input, and replies. It lists and gets the agents of the agent namespace, which it can create and update, and the built-in ones, of the builtin namespace, which can't be changed. Connectors' prompts aren't these. An agent which is created, or updated, is a tool from the next message."
}

func (*agentTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[agentInput]()
}

// Run does what the input asks, of the agents of the agent namespace
func (t *agentTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req agentInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, gollm.ErrBadParameter.With(err.Error())
		}
	}
	agents := t.agents()
	if agents == nil {
		return nil, gollm.ErrServiceUnavailable.With("there are no agents yet")
	}
	ref := strings.TrimSpace(req.Name)
	name := strings.TrimSpace(strings.TrimPrefix(ref, registryschema.AgentNamespace+":"))
	switch strings.TrimSpace(req.Action) {
	case "list":
		return listAgents(ctx, agents)
	case "get":
		if ref == "" {
			return nil, gollm.ErrBadParameter.With("missing agent name")
		}
		agent, err := agents.GetAgent(ctx, ref)
		if err != nil {
			return nil, err
		} else if !listed(agent) {
			return nil, gollm.ErrForbidden.Withf("agent %q is in the %s namespace, which isn't listed", ref, agent.Namespace())
		}
		toolkit.Logf(ctx, "Getting agent %s", ref)
		return agent.AgentMeta, nil
	case "create":
		if name == "" {
			return nil, gollm.ErrBadParameter.With("an agent needs a name")
		} else if strings.Contains(name, ":") {
			return nil, gollm.ErrForbidden.Withf("agents are only created in the %s namespace, so %q can't be", registryschema.AgentNamespace, name)
		} else if _, err := agents.GetAgent(ctx, name); err == nil {
			return nil, gollm.ErrConflict.Withf("agent %q exists, so it's updated, rather than created", name)
		} else if !errors.Is(err, gollm.ErrNotFound) {
			return nil, err
		}
		toolkit.Logf(ctx, "Creating agent %s", name)
		meta, err := req.meta(name)
		if err != nil {
			return nil, err
		}
		agent, _, err := agents.CreateAgent(ctx, meta)
		if err != nil {
			return nil, err
		}
		return summaryOf(agent), nil
	case "update":
		if _, err := ownAgent(ctx, agents, name); err != nil {
			return nil, err
		}
		toolkit.Logf(ctx, "Updating agent %s", name)
		meta, err := req.meta("")
		if err != nil {
			return nil, err
		}
		agent, err := agents.UpdateAgent(ctx, name, meta)
		if err != nil {
			return nil, err
		}
		return summaryOf(agent), nil
	}
	return nil, gollm.ErrBadParameter.Withf("action %q, which must be list, get, create or update", req.Action)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// meta returns the definition the input sets, with the name, when it has
// one, which an update hasn't
func (req agentInput) meta(name string) (registryschema.AgentMeta, error) {
	var meta registryschema.AgentMeta
	meta.Name, meta.Title, meta.Description, meta.Prompt = name, req.Title, req.Description, req.Prompt
	meta.System, meta.Model, meta.Tools = req.System, req.Model, req.Tools
	for _, field := range []struct {
		schema map[string]any
		raw    *json.RawMessage
	}{{req.Input, &meta.Input}, {req.Schema, &meta.Schema}} {
		if field.schema == nil {
			continue
		}
		data, err := json.Marshal(field.schema)
		if err != nil {
			return meta, gollm.ErrBadParameter.With(err.Error())
		}
		*field.raw = data
	}
	return meta, nil
}

// listAgents returns the agents which are listed, every page of them
func listAgents(ctx context.Context, agents Agents) ([]agentSummary, error) {
	result := []agentSummary{}
	for offset := uint(0); ; {
		list, err := agents.ListAgents(ctx, registryschema.AgentListRequest{Offset: offset})
		if err != nil {
			return nil, err
		}
		for i := range list.Body {
			if listed(&list.Body[i]) {
				result = append(result, summaryOf(&list.Body[i]))
			}
		}
		offset += uint(len(list.Body))
		if len(list.Body) == 0 || offset >= list.Count {
			return result, nil
		}
	}
}

// listed returns true for an agent of the agent namespace, or a built-in
// one which chats can call, rather than the ones only the server runs, such
// as builtin:event
func listed(agent *registryschema.Agent) bool {
	switch namespace := agent.Namespace(); namespace {
	case registryschema.AgentNamespace:
		return true
	case registryschema.BuiltinNamespace:
		return !toolkit.Unexpanded(namespace + ":" + agent.Name)
	}
	return false
}

// ownAgent returns the agent with the name, of the agent namespace, or
// ErrForbidden, when it's of another, such as a built-in one
func ownAgent(ctx context.Context, agents Agents, name string) (*registryschema.Agent, error) {
	if name == "" {
		return nil, gollm.ErrBadParameter.With("missing agent name")
	}
	agent, err := agents.GetAgent(ctx, name)
	if err != nil {
		return nil, err
	} else if namespace := agent.Namespace(); namespace != registryschema.AgentNamespace {
		return nil, gollm.ErrForbidden.Withf("agent %q is in the %s namespace, so it can't be changed", name, namespace)
	}
	return agent, nil
}

// summaryOf returns the agent, as it's listed
func summaryOf(agent *registryschema.Agent) agentSummary {
	return agentSummary{
		Name:        agent.Name,
		Namespace:   agent.Namespace(),
		Tool:        agent.Namespace() + ":" + agent.Name,
		Title:       types.Value(agent.Title),
		Description: types.Value(agent.Description),
		Model:       types.Value(agent.Model),
	}
}
