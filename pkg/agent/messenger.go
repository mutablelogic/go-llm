package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Send sends a single message and returns the response (stateless)
func (a *agent) WithoutSession(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Get the client for this model
	client := a.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no client found for model: %s", model.Name)
	}

	// Check if client implements Messenger
	messenger, ok := client.(llm.Messenger)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("client %q does not support messaging", client.Name())
	}

	// Send the message
	return messenger.WithoutSession(ctx, model, message, opts...)
}

// WithSession sends a message within a session and returns the response (stateful)
func (a *agent) WithSession(ctx context.Context, model schema.Model, session *schema.Session, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Get the client for this model
	client := a.clientForModel(model)
	if client == nil {
		return nil, llm.ErrNotFound.Withf("no client found for model: %s", model.Name)
	}

	// Check if client implements Messenger
	messenger, ok := client.(llm.Messenger)
	if !ok {
		return nil, llm.ErrNotImplemented.Withf("client %q does not support messaging", client.Name())
	}

	// Parse options to extract toolkit if provided
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Add toolkit tools to options if available
	toolkit, ok := options.GetToolkit().(*tool.Toolkit)
	if ok && toolkit != nil {
		if toolOpts, err := buildToolkitOpts(toolkit, client); err != nil {
			return nil, err
		} else {
			opts = append(toolOpts, opts...)
		}
	}

	for {
		// Send the message within the session
		resp, err := messenger.WithSession(ctx, model, session, message, opts...)
		if err != nil {
			return resp, err
		}

		// Run tools if any were used
		if results := runToolKitTools(ctx, toolkit, resp.ToolUse()); results == nil {
			return resp, nil
		} else {
			message = results
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// runTools runs the tools used in the message response and returns their results.
func runToolKitTools(ctx context.Context, toolkit *tool.Toolkit, tools []schema.ToolUse) *schema.Message {
	var wg sync.WaitGroup

	// Ignore if no tools or no toolkit
	if len(tools) == 0 || toolkit == nil {
		return nil
	}

	// Run tools in parallel
	message := schema.NewToolMessage()
	for _, tool := range tools {
		wg.Add(1)
		go func(t schema.ToolUse) {
			defer wg.Done()

			// Get the tool from the toolkit
			// TODO: Report on call to a tool
			results, err := toolkit.Run(ctx, *t.ToolName, t.ToolInput)
			if err != nil {
				message.AppendToolResult(t, err)
			} else {
				message.AppendToolResult(t, results)
			}
		}(tool)
	}

	// Wait for tools to complete
	wg.Wait()

	// Return the message
	return message
}

// buildToolkitOpts converts toolkit tools to option strings for downstream clients.
func buildToolkitOpts(toolkit *tool.Toolkit, client llm.Client) ([]opt.Opt, error) {
	if toolkit == nil {
		return nil, nil
	}

	toolOpts := make([]opt.Opt, 0)
	for _, t := range toolkit.Tools() {
		toolSchema, err := t.Schema()
		if err != nil {
			return nil, fmt.Errorf("failed to get schema for tool %q: %w", t.Name(), err)
		}

		def := schema.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: toolSchema,
		}

		// Prefer provider-specific tool optioner when available
		if to, ok := client.(llm.ToolOptioner); ok {
			optFn, err := to.ToolOption(def)
			if err != nil {
				return nil, err
			}
			toolOpts = append(toolOpts, optFn)
			continue
		}

		// Fallback: generic serialization
		data, err := json.Marshal(def)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize tool %q: %w", t.Name(), err)
		}
		toolOpts = append(toolOpts, opt.AddString("tools", string(data)))
	}

	return toolOpts, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS
