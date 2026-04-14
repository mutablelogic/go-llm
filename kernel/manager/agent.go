package manager

import (
	"context"
	"encoding/json"
	"slices"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	"github.com/mutablelogic/go-llm/pkg/opt"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	resource "github.com/mutablelogic/go-llm/toolkit/resource"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListAgents returns paginated prompt metadata from the current toolkit,
// exposing prompts externally as agents.
func (m *Manager) ListAgents(ctx context.Context, req schema.AgentListRequest, user *auth.User) (result *schema.AgentList, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListAgents",
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Filter prompts by namespace based on the user's accessible namespaces
	matched, count, err := m.listAgents(ctx, req, user)
	if err != nil {
		return nil, err
	}

	// Convert prompts to agent metadata
	body := make([]*schema.AgentMeta, 0, len(matched))
	for _, prompt := range matched {
		meta := newAgentMeta(prompt)
		body = append(body, &meta)
	}

	// Return the list response
	return &schema.AgentList{
		AgentListRequest: req,
		Count:            count,
		Body:             body,
	}, nil
}

// GetAgent returns agent metadata by name, scoped by the user's accessible namespaces.
func (m *Manager) GetAgent(ctx context.Context, name string, user *auth.User) (result *schema.AgentMeta, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "GetAgent",
		attribute.String("name", name),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Filter prompts by namespace based on the user's accessible namespaces, and return the one matching the given name
	prompts, _, err := m.listAgents(ctx, schema.AgentListRequest{Name: []string{name}}, user)
	if err != nil {
		return nil, err
	}

	// If there are no matches, return not found. If there are multiple matches, return a conflict error
	if len(prompts) == 0 {
		return nil, schema.ErrNotFound.Withf("agent %q", name)
	}
	if len(prompts) > 1 {
		return nil, schema.ErrConflict.Withf("multiple agents matched %q; specify a fully-qualified agent name", name)
	}

	// Convert the matched prompt to agent metadata and return it
	meta := newAgentMeta(prompts[0])
	return types.Ptr(meta), nil
}

// CallAgent executes an agent by name with the given input, scoped by the user's accessible namespaces.
func (m *Manager) CallAgent(ctx context.Context, name string, req schema.CallAgentRequest, user *auth.User) (result llm.Resource, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CallAgent",
		attribute.String("name", name),
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Filter prompts by namespace based on the user's accessible namespaces, and return the one matching the given name
	prompts, _, err := m.listAgents(ctx, schema.AgentListRequest{Name: []string{name}}, user)
	if err != nil {
		return nil, err
	}

	// There should be exactly one matching agent
	if len(prompts) == 0 {
		return nil, schema.ErrNotFound.Withf("agent %q", name)
	} else if len(prompts) > 1 {
		return nil, schema.ErrConflict.Withf("multiple agents matched %q; specify a fully-qualified agent name", name)
	}

	// Attach the JSON input as the first prompt resource if provided, then delegate prompt execution through the toolkit.
	resources := make([]llm.Resource, 0, len(req.Attachments)+1)
	if req.Input != nil {
		input, err := resource.JSON("input", json.RawMessage(req.Input))
		if err != nil {
			return nil, err
		}
		resources = append(resources, input)
	}
	for _, attachment := range req.Attachments {
		if attachment == nil {
			return nil, schema.ErrBadParameter.With("attachment cannot be nil")
		}
		resource, ok := any(attachment).(llm.Resource)
		if !ok {
			return nil, schema.ErrBadParameter.Withf("attachment must satisfy llm.Resource, got %T", attachment)
		}
		resources = append(resources, resource)
	}

	return m.Toolkit.Call(ctx, prompts[0], resources...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func newAgentMeta(prompt llm.Prompt) schema.AgentMeta {
	return schema.AgentMeta{
		Name:        prompt.Name(),
		Title:       prompt.Title(),
		Description: prompt.Description(),
	}
}

func (m *Manager) listAgents(ctx context.Context, req schema.AgentListRequest, user *auth.User) ([]llm.Prompt, uint, error) {
	var namespaces []string
	if user == nil {
		if req.Namespace != "" {
			namespaces = []string{req.Namespace}
		}
	} else {
		accessible, err := m.toolNamespacesForUser(ctx, user)
		if err != nil {
			return nil, 0, err
		}
		if req.Namespace == "" {
			namespaces = accessible
		} else if slices.Contains(accessible, req.Namespace) {
			namespaces = []string{req.Namespace}
		} else {
			return nil, 0, nil
		}
	}

	listReq := toolkit.ListRequest{
		Type:       toolkit.ListTypePrompts,
		Namespaces: namespaces,
		Name:       req.Name,
		Offset:     uint(req.Offset),
	}
	if req.Limit != nil {
		listReq.Limit = types.Ptr(uint(types.Value(req.Limit)))
	}

	resp, err := m.Toolkit.List(ctx, listReq)
	if err != nil {
		return nil, 0, err
	}

	return resp.Prompts, resp.Count, nil
}

func (m *Manager) runAgent(ctx context.Context, prompt llm.Prompt, content string, opts []opt.Opt, resources ...llm.Resource) (_ llm.Resource, err error) {
	// Extract the provider and the model from the prompt options
	agentopt, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	provider := agentopt.GetString(opt.ProviderKey)
	model := agentopt.GetString(opt.ModelKey)

	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "RunAgent",
		attribute.String("prompt", types.Stringify(prompt)),
		attribute.String("content", content),
		attribute.String("provider", types.Stringify(provider)),
		attribute.String("model", types.Stringify(model)),
	)
	defer func() { endSpan(err) }()

	// Not yet implemented
	return nil, schema.ErrNotImplemented.Withf("agent execution is not implemented for prompt %q, provider %q, model %q", prompt.Name(), provider, model)
}
