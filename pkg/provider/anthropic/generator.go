package anthropic

import (
	"context"
	"encoding/json"
	"io"

	// Packages
	"github.com/google/jsonschema-go/jsonschema"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession sends a single message and returns the response (stateless)
func (c *Client) WithoutSession(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	if message == nil {
		return nil, llm.ErrBadParameter.With("message is required")
	}
	session := schema.Session{message}
	return c.generate(ctx, model.Name, &session, opts...)
}

// WithSession sends a message within a session and returns the response (stateful)
func (c *Client) WithSession(ctx context.Context, model schema.Model, session *schema.Session, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	if session == nil {
		return nil, llm.ErrBadParameter.With("session is required")
	}
	if message == nil {
		return nil, llm.ErrBadParameter.With("message is required")
	}
	session.Append(*message)
	return c.generate(ctx, model.Name, session, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// generate is the core method that builds a request from options and sends it
func (c *Client) generate(ctx context.Context, model string, session *schema.Session, opts ...opt.Opt) (*schema.Message, error) {
	// Apply options
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	streamFn := options.GetStream()

	// Build request
	request, err := generateRequestFromOpts(model, session, options)
	if err != nil {
		return nil, err
	}

	// Force stream flag when streaming callback is set
	if streamFn != nil {
		request.Stream = true
	}

	// Create JSON payload
	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, err
	}

	// Streaming path
	if streamFn != nil {
		return c.generateStream(ctx, payload, session, streamFn)
	}

	// Non-streaming path
	var response messagesResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("messages")); err != nil {
		return nil, err
	}

	return c.processResponse(&response, session)
}

// generateStream handles the SSE streaming response from the Anthropic API
func (c *Client) generateStream(ctx context.Context, payload client.Payload, session *schema.Session, streamFn opt.StreamFn) (*schema.Message, error) {
	// Accumulators for building the final response
	var (
		role       string
		stopReason string
		usage      messagesUsage
		blocks     []anthropicContentBlock // one per content_block_start
		curIndex   int                     // index of the block currently being streamed
	)

	callback := func(event client.TextStreamEvent) error {
		// Parse the SSE data as JSON
		var ev streamEvent
		if err := event.Json(&ev); err != nil {
			return err
		}

		switch ev.Type {
		case eventMessageStart:
			if ev.Message != nil {
				role = ev.Message.Role
				usage = ev.Message.Usage
			}

		case eventContentBlockStart:
			curIndex = ev.Index
			if ev.ContentBlock != nil {
				// Grow the blocks slice to fit
				for len(blocks) <= curIndex {
					blocks = append(blocks, anthropicContentBlock{})
				}
				blocks[curIndex] = *ev.ContentBlock
				// Clear the Input field for tool_use blocks — the API sends
				// "input": {} as a placeholder, but the real content arrives
				// via input_json_delta events.
				if blocks[curIndex].Type == blockTypeToolUse {
					blocks[curIndex].Input = nil
				}
			}

		case eventContentBlockDelta:
			if ev.Delta == nil {
				break
			}
			// Grow the blocks slice if needed
			for len(blocks) <= ev.Index {
				blocks = append(blocks, anthropicContentBlock{})
			}

			switch ev.Delta.Type {
			case deltaTypeText:
				blocks[ev.Index].Text += ev.Delta.Text
				streamFn("assistant", ev.Delta.Text)
			case deltaTypeThinking:
				blocks[ev.Index].Thinking += ev.Delta.Thinking
				streamFn("thinking", ev.Delta.Thinking)
			case deltaTypeSignature:
				blocks[ev.Index].Signature += ev.Delta.Signature
			case deltaTypeInputJSON:
				// Accumulate partial JSON for tool_use input
				blocks[ev.Index].Input = append(blocks[ev.Index].Input, []byte(ev.Delta.PartialJSON)...)
			}
			curIndex = ev.Index

		case eventContentBlockStop:
			// Nothing to do — block is already accumulated

		case eventMessageDelta:
			if ev.Delta != nil {
				stopReason = ev.Delta.StopReason
			}
			if ev.Usage != nil {
				usage.OutputTokens = ev.Usage.OutputTokens
			}

		case eventMessageStop:
			return io.EOF // Signal end of stream

		case eventPing:
			// Ignore keepalive

		case eventError:
			// Return error from stream
			if ev.Delta != nil {
				return llm.ErrInternalServerError.Withf("stream error: %s", ev.Delta.Text)
			}
		}

		return nil
	}

	// Execute with streaming
	var discard messagesResponse
	if err := c.DoWithContext(ctx, payload, &discard, client.OptPath("messages"), client.OptTextStreamCallback(callback)); err != nil {
		return nil, err
	}

	// Refusal — no message to append
	if stopReason == stopReasonRefusal {
		return nil, llm.ErrRefusal
	}

	// Build final message from accumulated blocks
	message, err := messageFromAnthropicResponse(role, blocks, stopReason)
	if err != nil {
		return nil, err
	}

	// Append the message to the session with token counts
	session.AppendWithOuput(*message, usage.InputTokens, usage.OutputTokens)

	// Return error for stop reasons that need caller attention
	switch stopReason {
	case stopReasonMaxTokens:
		return message, llm.ErrMaxTokens
	case stopReasonPauseTurn:
		return message, llm.ErrPauseTurn
	}

	return message, nil
}

// processResponse handles the non-streaming response
func (c *Client) processResponse(response *messagesResponse, session *schema.Session) (*schema.Message, error) {
	// Refusal — no message to append
	if response.StopReason == stopReasonRefusal {
		return nil, llm.ErrRefusal
	}

	// Convert response to schema message
	message, err := messageFromAnthropicResponse(response.Role, response.Content, response.StopReason)
	if err != nil {
		return nil, err
	}

	// Append the message to the session with token counts
	session.AppendWithOuput(*message, response.Usage.InputTokens, response.Usage.OutputTokens)

	// Return error for stop reasons that need caller attention
	switch response.StopReason {
	case stopReasonMaxTokens:
		return message, llm.ErrMaxTokens
	case stopReasonPauseTurn:
		return message, llm.ErrPauseTurn
	}

	return message, nil
}

///////////////////////////////////////////////////////////////////////////////
// REQUEST BUILDING

// generateRequestFromOpts builds a messagesRequest from the session and applied options
func generateRequestFromOpts(model string, session *schema.Session, options opt.Options) (*messagesRequest, error) {
	// Convert session to Anthropic message format
	messages, err := anthropicMessagesFromSession(session)
	if err != nil {
		return nil, err
	}

	// Metadata
	var metadata *messagesMetadata
	if userId := options.GetString(opt.UserIdKey); userId != "" {
		metadata = &messagesMetadata{UserId: userId}
	}

	// System prompt (plain or cached)
	var system any
	if systemPrompt := options.GetString(opt.SystemPromptKey); systemPrompt != "" {
		if cacheControl := options.GetString(opt.CacheControlKey); cacheControl != "" {
			system = []textBlockParam{{
				Type:         "text",
				Text:         systemPrompt,
				CacheControl: &cacheControlEphemeral{Type: cacheControl},
			}}
		} else {
			system = systemPrompt
		}
	}

	// Temperature
	var temperature *float64
	if options.Has(opt.TemperatureKey) {
		v := options.GetFloat64(opt.TemperatureKey)
		temperature = &v
	}

	// Thinking config
	var thinking *thinkingConfig
	if options.Has(opt.ThinkingBudgetKey) {
		thinking = &thinkingConfig{
			Type:         "enabled",
			BudgetTokens: options.GetUint(opt.ThinkingBudgetKey),
		}
	}

	// Max tokens
	maxTokens := defaultMaxTokens
	if options.Has(opt.MaxTokensKey) {
		maxTokens = int(options.GetUint(opt.MaxTokensKey))
	}

	// Anthropic requires max_tokens > thinking.budget_tokens
	if thinking != nil && maxTokens <= int(thinking.BudgetTokens) {
		maxTokens = int(thinking.BudgetTokens) + defaultMaxTokens
	}

	// TopK
	var topK *uint
	if options.Has(opt.TopKKey) {
		v := options.GetUint(opt.TopKKey)
		topK = &v
	}

	// TopP
	var topP *float64
	if options.Has(opt.TopPKey) {
		v := options.GetFloat64(opt.TopPKey)
		topP = &v
	}

	// Output format (JSON schema)
	var outputFmt *outputFormat
	if schemaJSON := options.GetString(opt.JSONSchemaKey); schemaJSON != "" {
		var s jsonschema.Schema
		if err := json.Unmarshal([]byte(schemaJSON), &s); err == nil {
			outputFmt = &outputFormat{
				Type:       "json_schema",
				JSONSchema: &s,
			}
		}
	}

	// Tool choice
	var toolCh *toolChoice
	if tc := options.GetString(opt.ToolChoiceKey); tc != "" {
		toolCh = &toolChoice{Type: tc}
		if tc == "tool" {
			toolCh.Name = options.GetString(opt.ToolChoiceNameKey)
		}
	}

	// Tools from toolkit (agent path)
	var tools []json.RawMessage
	if v := options.Get(opt.ToolkitKey); v != nil {
		if tk, ok := v.(*tool.Toolkit); ok {
			toolsData, err := anthropicToolsFromToolkit(tk)
			if err != nil {
				return nil, err
			}
			tools = append(tools, toolsData...)
		}
	}

	return &messagesRequest{
		MaxTokens:     maxTokens,
		Messages:      messages,
		Metadata:      metadata,
		Model:         model,
		OutputConfig:  options.GetString(opt.OutputConfigKey),
		OutputFormat:  outputFmt,
		ServiceTier:   options.GetString(opt.ServiceTierKey),
		StopSequences: options.GetStringArray(opt.StopSequencesKey),
		System:        system,
		Temperature:   temperature,
		Thinking:      thinking,
		ToolChoice:    toolCh,
		Tools:         tools,
		TopK:          topK,
		TopP:          topP,
	}, nil
}

// GenerateRequest builds a generate request from options without sending it.
// Useful for testing and debugging.
func GenerateRequest(model string, session *schema.Session, opts ...opt.Opt) (any, error) {
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	return generateRequestFromOpts(model, session, options)
}
