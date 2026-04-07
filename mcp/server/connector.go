package server

import (
	"context"
	"encoding/json"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddConnector registers all tools, prompts, and resources currently exposed
// by an llm.Connector on this MCP server.
//
// AddConnector does not call conn.Run. Callers are responsible for managing
// the connector lifecycle when working with connectors that require an active
// background session.
func (s *Server) AddConnector(ctx context.Context, conn llm.Connector) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if conn == nil {
		return nil
	}

	tools, err := conn.ListTools(ctx)
	if err != nil {
		return err
	}
	if err := s.AddTools(tools...); err != nil {
		return err
	}

	prompts, err := conn.ListPrompts(ctx)
	if err != nil {
		return err
	}
	for _, prompt := range prompts {
		s.server.AddPrompt(promptFromPrompt(prompt), promptHandlerFromPrompt(prompt))
	}

	resources, err := conn.ListResources(ctx)
	if err != nil {
		return err
	}
	if err := s.AddResources(resources...); err != nil {
		return err
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func promptFromPrompt(prompt llm.Prompt) *sdkmcp.Prompt {
	return &sdkmcp.Prompt{
		Name:        prompt.Name(),
		Title:       prompt.Title(),
		Description: prompt.Description(),
	}
}

func promptHandlerFromPrompt(prompt llm.Prompt) sdkmcp.PromptHandler {
	return func(ctx context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		var raw json.RawMessage
		if req.Params != nil && len(req.Params.Arguments) > 0 {
			data, err := json.Marshal(req.Params.Arguments)
			if err != nil {
				return nil, err
			}
			raw = data
		}

		text, _, err := prompt.Prepare(ctx, raw)
		if err != nil {
			return nil, err
		}

		return &sdkmcp.GetPromptResult{
			Description: prompt.Description(),
			Messages: []*sdkmcp.PromptMessage{
				{
					Role:    "user",
					Content: &sdkmcp.TextContent{Text: text},
				},
			},
		}, nil
	}
}
