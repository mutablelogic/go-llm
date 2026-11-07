package registry

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// agentNamespace is the namespace of the toolkit's tools which are the
// registry's agents, by their names, which models call, such as
// agent__segment, so a model can ask an agent to do something. Its tools
// change as the agents do, which it tells the toolkit.
type agentNamespace struct {
	mu     sync.RWMutex
	tools  map[string]*agentTool
	notify func(eventschema.EventAction, string)
}

// agentTool is an agent, as a tool: its name, and what it does, which are
// the agent's, and its input and output, which are the agent's input, and
// the schema of its reply
type agentTool struct {
	registry *Registry
	agent    schema.Agent
	input    *jsonschema.Schema
	output   *jsonschema.Schema
	timeout  time.Duration
}

var _ toolkit.Namespace = (*agentNamespace)(nil)
var _ llm.Tool = (*agentTool)(nil)
var _ toolkit.ToolTitler = (*agentTool)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// AgentNamespace is the name of the namespace of the toolkit's tools
	// which are the registry's agents
	AgentNamespace = "agent"

	// agentPrompt is the system prompt of the agent namespace, which says
	// how to use its tools
	agentPrompt = "The tools in the agent namespace are agents: each asks a model to do what its description says, with its own instructions, and returns its reply. Call one to delegate a task it does."
)

// emptyInput is the input schema of an agent which has none
var emptyInput = jsonschema.MustFor[struct{}]()

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newAgentNamespace returns the namespace of the registry's agents, which
// has none
func newAgentNamespace() *agentNamespace {
	return &agentNamespace{tools: make(map[string]*agentTool)}
}

///////////////////////////////////////////////////////////////////////////////
// NAMESPACE

func (n *agentNamespace) Name() string {
	return AgentNamespace
}

// Description returns what the agent namespace is
func (n *agentNamespace) Description() string {
	return "Agents, which each ask a model to do a task, with their own instructions, and tools, and reply"
}

func (n *agentNamespace) SystemPrompt() string {
	return agentPrompt
}

// Tools returns the agents, as tools, in order of their names
func (n *agentNamespace) Tools() []llm.Tool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	result := make([]llm.Tool, 0, len(n.tools))
	for _, tool := range n.tools {
		result = append(result, tool)
	}
	slices.SortFunc(result, func(a, b llm.Tool) int { return cmp.Compare(a.Name(), b.Name()) })
	return result
}

// Notify sets the function which is called when an agent is created,
// updated or deleted
func (n *agentNamespace) Notify(fn func(eventschema.EventAction, string)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notify = fn
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// set adds the agent, as a tool, or replaces it, and tells the toolkit. An
// agent whose input, or schema, isn't valid, which the registry doesn't
// store, or which is internal, as only the server runs it, isn't a tool.
func (n *agentNamespace) set(r *Registry, agent schema.Agent) {
	if types.Value(agent.Internal) {
		n.remove(agent.Name)
		return
	}
	tool, err := newAgentTool(r, agent)
	if err != nil {
		n.remove(agent.Name)
		return
	}
	n.mu.Lock()
	_, exists := n.tools[agent.Name]
	n.tools[agent.Name] = tool
	notify := n.notify
	n.mu.Unlock()

	// The toolkit is told without the lock, as it reads the tools
	if notify != nil {
		if exists {
			notify(eventschema.EventUpdate, agent.Name)
		} else {
			notify(eventschema.EventCreate, agent.Name)
		}
	}
}

// remove removes the agent with the name, as a tool, and tells the toolkit
func (n *agentNamespace) remove(name string) {
	n.mu.Lock()
	_, exists := n.tools[name]
	delete(n.tools, name)
	notify := n.notify
	n.mu.Unlock()
	if exists && notify != nil {
		notify(eventschema.EventDelete, name)
	}
}

///////////////////////////////////////////////////////////////////////////////
// TOOL

// newAgentTool returns the agent, as a tool, or an error when its input, or
// the schema of its reply, isn't valid
func newAgentTool(r *Registry, agent schema.Agent) (*agentTool, error) {
	input, err := agent.InputSchema()
	if err != nil {
		return nil, err
	}
	output, err := agent.JSONSchema()
	if err != nil {
		return nil, err
	}
	timeout, err := agent.ToolTimeout()
	if err != nil {
		return nil, err
	}
	return &agentTool{registry: r, agent: agent, input: input, output: output, timeout: timeout}, nil
}

func (t *agentTool) Name() string {
	return t.agent.Name
}

// Title returns the agent's title, for people, or empty, when it has none
func (t *agentTool) Title() string {
	return types.Value(t.agent.Title)
}

// Description returns what the agent does, or its title, or that it's an
// agent, when it has neither
func (t *agentTool) Description() string {
	if description := types.Value(t.agent.Description); description != "" {
		return description
	} else if title := types.Value(t.agent.Title); title != "" {
		return title
	}
	return "The agent " + t.agent.Name
}

// InputSchema returns the agent's input, or an empty object, when it has
// none
func (t *agentTool) InputSchema() *jsonschema.Schema {
	if t.input == nil {
		return emptyInput
	}
	return t.input
}

// OutputSchema returns the schema of the agent's reply, or nil, when its
// reply is text
func (t *agentTool) OutputSchema() *jsonschema.Schema {
	return t.output
}

// Timeout returns the agent's timeout, or zero, for the time a tool has by
// default, when it has none
func (t *agentTool) Timeout() time.Duration {
	return t.timeout
}

// Run runs the agent, with the input, from the session of the model which
// called it, when there's one, and returns its reply, which is its text, or
// what it replied, when it has a schema, or an error, which says how it
// stopped, when its reply isn't complete. What the agent's tools log, and
// their progress, are its own, and that it's thinking, or replying, is its
// progress, so they're streamed, as it runs, however deep it is.
func (t *agentTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	session := toolkit.SessionFromContext(ctx)
	req := schema.RunAgentRequest{Input: input, Session: session.ID()}
	response, err := t.registry.runAgent(ctx, t.agent.Name, req, agentStream(t.agent.Name, session), true)
	if err != nil {
		return nil, err
	}

	// The tokens it used are the tool's, which its result has, even when it
	// didn't finish
	session.Usage(response.Usage.Usage)
	if err := agentFinished(t.agent.Name, response.Messages); err != nil {
		return nil, err
	}
	return response.Reply, nil
}

// agentStream returns the function which streams what the agent with the
// name streams, as it runs, to the session it's run in: what its tools log,
// and their progress, as they are, and that it's thinking, or replying, as
// its progress, once each time it starts, rather than each piece. Its
// progress is a count, of its own, and its tools', which increases, as MCP
// says it does.
func agentStream(name string, session toolkit.Session) llm.GeneratorStreamFn {
	var last llm.StreamType
	var count float64
	return func(what llm.StreamType, text string) {
		switch what {
		case llm.StreamLog:
			// The agent's next thinking, or reply, is said again, after it
			session.Logger().Info(text)
			last = what
		case llm.StreamProgress:
			// Its tools' progress is counted as its own, so its progress
			// increases, whose total, which is of the tool's own count, isn't
			// known, and its next thinking, or reply, is said again, after it
			count++
			session.Progress(count, 0, llm.ParseProgress(text).Message)
			last = what
		case llm.StreamThinking, llm.StreamText:
			if what == last {
				return
			}
			last = what
			count++
			if what == llm.StreamThinking {
				session.Progress(count, 0, "Agent "+name+" is thinking")
			} else {
				session.Progress(count, 0, "Agent "+name+" is replying")
			}
		}
	}
}
