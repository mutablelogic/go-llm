package manager

import (
	"context"
	"encoding/json"
	"sync"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	anthropic "github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	eliza "github.com/mutablelogic/go-llm/pkg/provider/eliza"
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
	session, err := m.sessionStore.GetSession(ctx, request.Session)
	if err != nil {
		return nil, err
	}

	// Resolve model, generator, and options from the session meta.
	// If the request includes a per-request system prompt, merge it with
	// the session's own system prompt so callers (like the Telegram bot)
	// can inject formatting instructions on every call.
	meta := session.GeneratorMeta
	if request.SystemPrompt != "" {
		if meta.SystemPrompt != "" {
			meta.SystemPrompt += "\n\n" + request.SystemPrompt
		} else {
			meta.SystemPrompt = request.SystemPrompt
		}
	}

	model, generator, opts, err := m.generatorFromMeta(ctx, meta)
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

	// When both tools and a JSON schema are present, convert the schema
	// into a "submit_output" tool so the model can call tools AND produce
	// structured output (some providers like Gemini can't combine function
	// calling with a response JSON schema).
	var outputToolName string
	var outputToolOpt opt.Opt
	var outputTool *tool.OutputTool
	if toolkitOpt != nil && len(meta.Format) > 0 {
		outputToolName, outputTool, outputToolOpt, err = m.addOutputTool(meta.Format)
		if err != nil {
			return nil, err
		}
		// Rebuild opts without the JSON schema but with the output-tool
		// instruction appended to the system prompt.
		metaNoFormat := meta
		metaNoFormat.Format = nil
		if metaNoFormat.SystemPrompt != "" {
			metaNoFormat.SystemPrompt += "\n\n"
		}
		metaNoFormat.SystemPrompt += tool.OutputToolInstruction
		_, _, opts, err = m.generatorFromMeta(ctx, metaNoFormat)
		if err != nil {
			return nil, err
		}
		// Wrap the stream callback to suppress assistant text when the
		// output tool is active — the model may "think out loud" before
		// calling submit_output.  We still forward tool and thinking roles.
		if fn != nil {
			wrappedFn := opt.StreamFn(func(role, text string) {
				if role != schema.RoleAssistant {
					fn(role, text)
				}
			})
			opts = append(opts, opt.WithStream(wrappedFn))
		}
	}
	if toolkitOpt != nil {
		opts = append(opts, toolkitOpt)
	}
	if outputToolOpt != nil {
		opts = append(opts, outputToolOpt)
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

		// Check if any tool call is the output tool — if so, capture it
		// as structured output and stop the loop.
		if outputToolName != "" {
			for _, call := range toolCalls {
				if call.Name == outputToolName {
					// Validate the output against the schema
					if outputTool != nil {
						if err := outputTool.Validate(call.Input); err != nil {
							return nil, llm.ErrBadParameter.Withf("%v", err)
						}
					}
					// The tool's input IS the structured output.
					// Replace the last message in the conversation (the model's
					// tool-call response) with a clean assistant message containing
					// only the JSON output. This avoids leaving an orphaned tool
					// call in the history. Preserve the original token count so
					// overhead calculations remain correct.
					text := string(call.Input)
					var tokens uint
					if n := len(session.Messages); n > 0 {
						tokens = session.Messages[n-1].Tokens
					}
					result = &schema.Message{
						Role: schema.RoleAssistant,
						Content: []schema.ContentBlock{
							{Text: &text},
						},
						Result: schema.ResultStop,
						Tokens: tokens,
					}
					if n := len(session.Messages); n > 0 {
						session.Messages[n-1] = result
					}
					goto done
				}
			}
		}

		// Execute tool calls in parallel then build a tool-result message and send it back
		toolMessage := &schema.Message{
			Role:    schema.RoleUser,
			Content: m.runTools(ctx, toolCalls, fn),
		}

		// Send the tool results back to the model for the next iteration
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

done:
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
	if err := m.sessionStore.WriteSession(session); err != nil {
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
	} else if meta.Thinking != nil && *meta.Thinking {
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
		case schema.Eliza:
			return eliza.WithThinking()
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
		case schema.Eliza:
			return eliza.WithThinking()
		default:
			return opt.Error(llm.ErrNotImplemented.Withf("%s: WithThinking not supported", provider))
		}
	})
}

// withJSONOutput dispatches to the correct provider-specific JSON output option.
func withJSONOutput(data schema.JSONSchema) opt.Opt {
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

// addOutputTool creates a "submit_output" tool whose parameter schema matches
// the provided JSON schema, and returns an opt.Opt that adds it to the request.
// This allows the model to produce structured output by calling this tool,
// avoiding the conflict between function calling and response JSON schema on
// providers like Gemini. Returns the tool name and the opt.
func (m *Manager) addOutputTool(format schema.JSONSchema) (string, *tool.OutputTool, opt.Opt, error) {
	var s jsonschema.Schema
	if err := json.Unmarshal(format, &s); err != nil {
		return "", nil, nil, llm.ErrBadParameter.Withf("invalid JSON schema for output tool: %v", err)
	}
	outputTool := tool.NewOutputTool(&s)
	return tool.OutputToolName, outputTool, tool.WithTool(outputTool), nil
}

// runTools executes the given tool calls in parallel and returns the results
// as content blocks in the same order as the input calls. If fn is non-nil,
// tool feedback is streamed before execution begins.
func (m *Manager) runTools(ctx context.Context, calls []schema.ToolCall, fn opt.StreamFn) []schema.ContentBlock {
	results := make([]schema.ContentBlock, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		if fn != nil {
			fn(schema.RoleTool, m.toolkit.Feedback(call))
		}
		wg.Add(1)
		go func(i int, call schema.ToolCall) {
			defer wg.Done()
			output, err := m.toolkit.Run(ctx, call.Name, call.Input)
			if err != nil {
				results[i] = schema.NewToolError(call.ID, call.Name, err)
			} else {
				results[i] = schema.NewToolResult(call.ID, call.Name, output)
			}
		}(i, call)
	}
	wg.Wait()
	return results
}

// withTools returns an opt that sets a toolkit for the request.
// If no tool names are provided (nil slice), all tools from the manager's
// toolkit are included. If an empty non-nil slice is passed, no tools are
// included. If specific names are given, only those tools are included.
func (m *Manager) withTools(tools ...string) (opt.Opt, error) {
	if tools == nil {
		// No filter — include all tools
		if len(m.toolkit.Tools()) == 0 {
			return nil, nil
		}
		return tool.WithToolkit(m.toolkit), nil
	}

	// Explicitly empty — no tools requested
	if len(tools) == 0 {
		return nil, nil
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
