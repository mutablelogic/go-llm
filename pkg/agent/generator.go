package agent

import (
	"context"
	"encoding/json"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	anthropic "github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ask processes a message and returns a response, outside of a session context (stateless).
// If fn is non-nil, text chunks are streamed to the callback as they arrive.
func (m *Manager) Ask(ctx context.Context, request schema.AskRequest, fn opt.StreamFn) (*schema.AskResponse, error) {
	// Resolve model, generator, and options from the request meta
	model, generator, opts, err := m.generatorFromMeta(ctx, request.GeneratorMeta)
	if err != nil {
		return nil, err
	}

	// Enable streaming when a callback is provided
	if fn != nil {
		opts = append(opts, opt.WithStream(fn))
	}

	// Build message options from attachments
	var msgOpts []opt.Opt
	for i := range request.Attachments {
		a := request.Attachments[i]
		msgOpts = append(msgOpts, opt.AddAny(opt.ContentBlockKey, schema.ContentBlock{
			Attachment: &a,
		}))
	}

	// Create the user message
	message, err := schema.NewMessage(schema.RoleUser, request.Text, msgOpts...)
	if err != nil {
		return nil, err
	}

	// Send the message
	result, usage, err := generator.WithoutSession(ctx, *model, message, opts...)
	if err != nil {
		return nil, err
	}

	// Return the response
	response := &schema.AskResponse{
		CompletionResponse: schema.CompletionResponse{
			Role:    result.Role,
			Content: result.Content,
			Result:  result.Result,
		},
		Usage: usage,
	}
	return response, nil
}

// Chat processes a message within a session context (stateful).
// If fn is non-nil, text chunks are streamed to the callback as they arrive.
func (m *Manager) Chat(ctx context.Context, request schema.ChatRequest, fn opt.StreamFn) (*schema.ChatResponse, error) {
	// Retrieve the session
	session, err := m.store.Get(ctx, request.Session)
	if err != nil {
		return nil, err
	}

	// Resolve model, generator, and options from the session meta
	model, generator, opts, err := m.generatorFromMeta(ctx, session.GeneratorMeta)
	if err != nil {
		return nil, err
	}

	// Enable streaming when a callback is provided
	if fn != nil {
		opts = append(opts, opt.WithStream(fn))
	}

	// Include tools in the request
	toolkitOpt, err := m.withTools(request.Tools...)
	if err != nil {
		return nil, err
	}
	if toolkitOpt != nil {
		opts = append(opts, toolkitOpt)
	}

	// Build message options from attachments
	var msgOpts []opt.Opt
	for i := range request.Attachments {
		a := request.Attachments[i]
		msgOpts = append(msgOpts, opt.AddAny(opt.ContentBlockKey, schema.ContentBlock{
			Attachment: &a,
		}))
	}

	// Create the user message
	message, err := schema.NewMessage(schema.RoleUser, request.Text, msgOpts...)
	if err != nil {
		return nil, err
	}

	// Send the message within the session
	result, usage, err := generator.WithSession(ctx, *model, session.Conversation(), message, opts...)
	if err != nil {
		return nil, err
	}

	// Tool-calling loop: if the model requests tool calls, execute them
	// and feed results back until we get a final response or hit the limit.
	// Snapshot the conversation length so we can roll back if we exhaust iterations.
	maxIter := request.MaxIterations
	if maxIter == 0 {
		maxIter = schema.DefaultMaxIterations
	}
	msgSnapshot := len(session.Messages)
	for i := uint(0); i < maxIter && result.Result == schema.ResultToolCall; i++ {
		toolCalls := result.ToolCalls()
		if len(toolCalls) == 0 {
			break
		}

		// Execute each tool call and collect result blocks
		var toolResults []schema.ContentBlock
		for _, call := range toolCalls {
			output, err := m.toolkit.Run(ctx, call.Name, call.Input)
			if err != nil {
				toolResults = append(toolResults, schema.NewToolError(call.ID, call.Name, err))
			} else {
				toolResults = append(toolResults, schema.NewToolResult(call.ID, call.Name, output))
			}
		}

		// Build a tool-result message and send it back
		toolMessage := &schema.Message{
			Role:    schema.RoleUser,
			Content: toolResults,
		}
		var u *schema.Usage
		result, u, err = generator.WithSession(ctx, *model, session.Conversation(), toolMessage, opts...)
		if err != nil {
			return nil, err
		}

		// Accumulate usage
		if u != nil {
			if usage == nil {
				usage = u
			} else {
				usage.InputTokens += u.InputTokens
				usage.OutputTokens += u.OutputTokens
			}
		}
	}

	// If we exhausted the iteration limit while the model still wants
	// tool calls, roll back the conversation and report the condition.
	if result.Result == schema.ResultToolCall {
		session.Messages = session.Messages[:msgSnapshot]
		result.Result = schema.ResultMaxIterations
	}

	// Calculate per-turn overhead (tool schemas, system prompt, etc.)
	// as the difference between actual input tokens and the sum of
	// estimated/recorded message content tokens.
	if usage != nil && usage.InputTokens > 0 {
		// Input tokens cover all messages except the latest response
		inputMsgTokens := session.Messages.Tokens() - result.Tokens
		if usage.InputTokens > inputMsgTokens {
			session.Overhead = usage.InputTokens - inputMsgTokens
		}
	}

	// Persist the updated session
	if err := m.store.Write(session); err != nil {
		return nil, err
	}

	// Return the response
	response := &schema.ChatResponse{
		CompletionResponse: schema.CompletionResponse{
			Role:    result.Role,
			Content: result.Content,
			Result:  result.Result,
		},
		Session: session.ID,
		Usage:   usage,
	}
	return response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// generatorFromMeta resolves the model and generator client from the given
// GeneratorMeta, and returns provider-specific options derived from the meta
// fields (e.g. system prompt). This is reusable for both Ask and Chat.
func (m *Manager) generatorFromMeta(ctx context.Context, meta schema.GeneratorMeta) (*schema.Model, llm.Generator, []opt.Opt, error) {
	// Get the model
	model, err := m.getModel(ctx, meta.Provider, meta.Model)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get the client for the model
	client := m.clientForModel(model)
	if client == nil {
		return nil, nil, nil, llm.ErrNotFound.Withf("no provider found for model: %s", meta.Model)
	}
	generator, ok := client.(llm.Generator)
	if !ok {
		return nil, nil, nil, llm.ErrNotImplemented.Withf("provider %q does not support messaging", client.Name())
	}

	// Build options from meta fields
	var opts []opt.Opt
	if meta.SystemPrompt != "" {
		opts = append(opts, withSystemPrompt(meta.SystemPrompt))
	}
	if len(meta.Format) > 0 {
		opts = append(opts, withJSONOutput(meta.Format))
	}
	if meta.ThinkingBudget > 0 {
		opts = append(opts, withThinkingBudget(meta.ThinkingBudget))
	} else if meta.Thinking {
		opts = append(opts, withThinking())
	}

	// Convert options for the client
	opts, err = convertOptsForClient(opts, client)
	if err != nil {
		return nil, nil, nil, err
	}

	return model, generator, opts, nil
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

// withThinking dispatches to the correct provider-specific thinking option (no budget).
func withThinking() opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithThinking()
		default:
			return opt.Error(llm.ErrBadParameter.Withf("%s: WithThinking without budget not supported (use --thinking-budget)", provider))
		}
	})
}

// withThinkingBudget dispatches to the correct provider-specific thinking option with a token budget.
func withThinkingBudget(budgetTokens uint) opt.Opt {
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithThinkingBudget(budgetTokens)
		case schema.Anthropic:
			return anthropic.WithThinking(budgetTokens)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithThinking not supported", provider))
		}
	})
}

// withJSONOutput dispatches to the correct provider-specific JSON output option.
func withJSONOutput(data json.RawMessage) opt.Opt {
	var s jsonschema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return opt.Error(llm.ErrBadParameter.Withf("invalid JSON schema: %v", err))
	}
	return opt.WithClient(func(provider string) opt.Opt {
		switch provider {
		case schema.Gemini:
			return google.WithJSONOutput(&s)
		case schema.Anthropic:
			return anthropic.WithJSONOutput(&s)
		case schema.Mistral:
			return mistral.WithJSONOutput(&s)
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithJSONOutput not supported", provider))
		}
	})
}

// withTools returns an opt that sets a toolkit for the request. If tool names
// are provided, only those tools are included; otherwise all tools from the
// manager's toolkit are included. Returns nil if the toolkit is empty.
func (m *Manager) withTools(tools ...string) (opt.Opt, error) {
	if len(tools) == 0 {
		// No filter — include all tools
		if len(m.toolkit.Tools()) == 0 {
			return nil, nil
		}
		return tool.WithToolkit(m.toolkit), nil
	}

	// Build a filtered toolkit with only the requested tools
	filtered := make([]tool.Tool, 0, len(tools))
	for _, name := range tools {
		t := m.toolkit.Lookup(name)
		if t == nil {
			return nil, llm.ErrNotFound.Withf("tool %q", name)
		}
		filtered = append(filtered, t)
	}

	tk, err := tool.NewToolkit(filtered...)
	if err != nil {
		return nil, err
	}
	return tool.WithToolkit(tk), nil
}
