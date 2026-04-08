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

// AddPrompts registers one or more llm.Prompt values as MCP prompts.
func (s *Server) AddPrompts(prompts ...llm.Prompt) {
	for _, prompt := range prompts {
		s.server.AddPrompt(promptFromPrompt(prompt), promptHandlerFromPrompt(prompt))
	}
}

// RemovePrompts removes the named prompts from the server. Unknown names are
// silently ignored.
func (s *Server) RemovePrompts(names ...string) {
	s.server.RemovePrompts(names...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func promptFromPrompt(prompt llm.Prompt) *sdkmcp.Prompt {
	result := &sdkmcp.Prompt{
		Name:        prompt.Name(),
		Title:       prompt.Title(),
		Description: prompt.Description(),
	}
	if args := promptArgumentsFromPrompt(prompt); len(args) > 0 {
		result.Arguments = args
	}
	return result
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

func promptArgumentsFromPrompt(prompt llm.Prompt) []*sdkmcp.PromptArgument {
	data, err := json.Marshal(prompt)
	if err != nil {
		return nil
	}

	var decoded struct {
		Arguments []*sdkmcp.PromptArgument `json:"arguments,omitempty"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil
	}
	return decoded.Arguments
}
