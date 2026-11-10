package registry

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	agentschema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// SourceAgent returns the definition of the connector's MCP prompt, as an
// agent, by its namespace, which is the connector's tools', and its name as
// an agent, such as args_prompt, with its namespace as its origin, so the
// registry runs it as it runs its agents. Returns ErrNotFound when there's
// no connector in the namespace, it isn't connected, or it has no such
// prompt.
func (r *Registry) SourceAgent(_ context.Context, ns, name string) (agentschema.AgentMeta, error) {
	n, err := r.agentNamespace(ns)
	if err != nil {
		return agentschema.AgentMeta{}, err
	}
	p, err := n.Prompt(name)
	if err != nil {
		return agentschema.AgentMeta{}, err
	}
	var meta agentschema.AgentMeta
	meta.Name, meta.Origin = name, ns
	if title := strings.TrimSpace(p.Title); title != "" {
		meta.Title = types.Ptr(title)
	}
	if description := strings.TrimSpace(p.Description); description != "" {
		meta.Description = types.Ptr(description)
	}
	for _, tool := range n.Tools() {
		if tool.Name() == name {
			if input, err := json.Marshal(tool.InputSchema()); err == nil {
				meta.Input = input
			}
		}
	}
	return meta, nil
}

// SourceMessages returns the messages of the connector's MCP prompt, by its
// namespace, and its name as an agent, with the input, which are its
// arguments, as its model is sent them. Returns ErrNotFound when there's no
// such prompt, and ErrBadParameter when the input isn't valid.
func (r *Registry) SourceMessages(ctx context.Context, ns, name string, input json.RawMessage) ([]llm.Message, error) {
	n, err := r.agentNamespace(ns)
	if err != nil {
		return nil, err
	}
	return n.PromptMessages(ctx, name, input)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// agentNamespace returns the connector's namespace with the name, which its
// prompts are agents in, or ErrNotFound when there's none
func (r *Registry) agentNamespace(name string) (*namespace.Namespace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ns := range r.namespaces {
		if ns.Name() == name {
			return ns, nil
		}
	}
	return nil, llm.ErrNotFound.Withf("no connector's namespace %q", name)
}
