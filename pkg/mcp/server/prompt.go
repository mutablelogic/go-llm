package server

import (
	"context"
	"encoding/json"
	"sort"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddPrompts registers one or more agents as MCP prompts. Each prompt's
// arguments are derived from the AgentMeta's Input JSON Schema, and the
// handler renders the agent's template with the arguments supplied by the client.
func (s *Server) AddPrompts(metas ...schema.AgentMeta) {
	for _, meta := range metas {
		s.server.AddPrompt(promptFromAgentMeta(meta), promptHandlerFromAgentMeta(meta))
	}
}

// RemovePrompts removes the named prompts from the server. Unknown names are
// silently ignored.
func (s *Server) RemovePrompts(names ...string) {
	s.server.RemovePrompts(names...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE TYPES

// schemaDoc is a minimal JSON Schema document used only to extract property
// names, descriptions, and required flags for MCP PromptArguments.
type schemaDoc struct {
	Properties map[string]schemaProperty `json:"properties"`
	Required   []string                  `json:"required"`
}

type schemaProperty struct {
	Description string `json:"description"`
	Title       string `json:"title"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// promptFromAgentMeta converts an AgentMeta into an *sdkmcp.Prompt. The
// Prompt's Arguments are derived from the top-level properties of the
// AgentMeta.Input JSON Schema.
func promptFromAgentMeta(meta schema.AgentMeta) *sdkmcp.Prompt {
	return &sdkmcp.Prompt{
		Name:        meta.Name,
		Title:       meta.Title,
		Description: meta.Description,
		Arguments:   argsFromJSONSchema(meta.Input),
	}
}

// promptHandlerFromAgentMeta returns an sdkmcp.PromptHandler that renders the
// agent template with the arguments supplied by the MCP client.
func promptHandlerFromAgentMeta(meta schema.AgentMeta) sdkmcp.PromptHandler {
	a := &schema.Agent{AgentMeta: meta}
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		// Marshal the client's map[string]string arguments to JSON so that
		// agent.Prepare can validate them against the input schema and render
		// the template.
		var raw json.RawMessage
		if req.Params != nil && len(req.Params.Arguments) > 0 {
			var err error
			if raw, err = json.Marshal(req.Params.Arguments); err != nil {
				return nil, err
			}
		}

		result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, raw)
		if err != nil {
			return nil, err
		}

		return &sdkmcp.GetPromptResult{
			Description: meta.Description,
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: result.Text},
				},
			},
		}, nil
	}
}

// argsFromJSONSchema parses the top-level properties of a JSON Schema and
// returns them as MCP PromptArguments. Unknown or empty schemas return nil.
func argsFromJSONSchema(s schema.JSONSchema) []*sdkmcp.PromptArgument {
	if len(s) == 0 {
		return nil
	}
	var doc schemaDoc
	if err := json.Unmarshal(s, &doc); err != nil || len(doc.Properties) == 0 {
		return nil
	}

	required := make(map[string]bool, len(doc.Required))
	for _, r := range doc.Required {
		required[r] = true
	}

	args := make([]*sdkmcp.PromptArgument, 0, len(doc.Properties))
	names := make([]string, 0, len(doc.Properties))
	for name := range doc.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		prop := doc.Properties[name]
		desc := prop.Description
		if desc == "" {
			desc = prop.Title
		}
		args = append(args, &sdkmcp.PromptArgument{
			Name:        name,
			Description: desc,
			Required:    required[name],
		})
	}
	return args
}
