package agent

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	anthropic "github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	defaultMaxIterations = 10 // Default guard against infinite tool-calling loops
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ask sends a single message to the model using the session's configuration
// (provider, model, system prompt) but does NOT store anything in the session.
func (m *Manager) Ask(ctx context.Context, session *schema.Session, message *schema.Message) (*schema.Message, error) {
	// Get the model
	model, err := m.getModel(ctx, session.Provider, session.Model)
	if err != nil {
		return nil, err
	}

	// Get the client for this model
	client := m.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no client found for model: %s", model.Name)
	}

	// Check if client implements Generator
	generator, ok := client.(llm.Generator)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("client %q does not support generation", client.Name())
	}

	// Build options
	opts, err := m.generationOpts(client, session)
	if err != nil {
		return nil, err
	}

	// Send the message (stateless)
	return generator.WithoutSession(ctx, *model, message, opts...)
}

// Chat sends a message within a session's conversation, stores the exchange,
// and handles tool-call loops if a toolkit is configured.
func (m *Manager) Chat(ctx context.Context, session *schema.Session, message *schema.Message) (*schema.Message, error) {
	// Get the model
	model, err := m.getModel(ctx, session.Provider, session.Model)
	if err != nil {
		return nil, err
	}

	// Get the client for this model
	client := m.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no client found for model: %s", model.Name)
	}

	// Check if client implements Generator
	generator, ok := client.(llm.Generator)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("client %q does not support generation", client.Name())
	}

	// Build options
	opts, err := m.generationOpts(client, session)
	if err != nil {
		return nil, err
	}

	// Add incoming message to the session
	session.Append(*message)

	// Send the message within the conversation
	conversation := session.Conversation()
	resp, err := generator.WithSession(ctx, *model, conversation, message, opts...)
	if err != nil {
		return nil, err
	}

	// Tool-call loop
	for i := 0; m.toolkit != nil && resp.Result == schema.ResultToolCall && i < defaultMaxIterations; i++ {
		calls := resp.ToolCalls()
		if len(calls) == 0 {
			break
		}

		// Execute each tool call and collect result blocks
		results := make([]schema.ContentBlock, 0, len(calls))
		for _, call := range calls {
			if result, err := m.toolkit.Run(ctx, call.Name, call.Input); err != nil {
				results = append(results, schema.NewToolError(call.ID, call.Name, err))
			} else {
				results = append(results, schema.NewToolResult(call.ID, call.Name, result))
			}
		}

		// Feed tool results back to the model
		resp, err = generator.WithSession(ctx, *model, conversation, &schema.Message{
			Role:    schema.RoleUser,
			Content: results,
		}, opts...)
		if err != nil {
			return nil, err
		}
	}

	// If the loop ended because we hit the iteration limit, report an error
	if m.toolkit != nil && resp.Result == schema.ResultToolCall {
		return nil, llm.ErrInternalServerError.Withf("tool call loop did not resolve after %d iterations", defaultMaxIterations)
	}

	// Persist the session
	if err := m.store.Write(session); err != nil {
		return nil, err
	}

	return resp, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// generationOpts builds the option slice for a generation call, applying
// the session's system prompt and the manager's toolkit.
func (m *Manager) generationOpts(client llm.Client, session *schema.Session) ([]opt.Opt, error) {
	var opts []opt.Opt

	// Add system prompt if set - dispatch to provider-specific option
	if session.SystemPrompt != "" {
		opts = append(opts, withSystemPrompt(session.SystemPrompt))
	}

	// Add toolkit if configured
	if m.toolkit != nil {
		opts = append(opts, tool.WithToolkit(m.toolkit))
	}

	// Convert options for the specific client/provider
	return convertOptsForClient(opts, client)
}

// withSystemPrompt dispatches to the correct provider-specific system prompt option.
func withSystemPrompt(value string) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithSystemPrompt(value)
		case schema.Anthropic:
			return anthropic.WithSystemPrompt(value)
		case schema.Mistral:
			return mistral.WithSystemPrompt(value)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithSystemPrompt not supported", provider))
		}
	})
}
