package mistral

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
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
	session := schema.Conversation{message}
	return c.generate(ctx, model.Name, &session, opts...)
}

// WithSession sends a message within a session and returns the response (stateful)
func (c *Client) WithSession(ctx context.Context, model schema.Model, session *schema.Conversation, message *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
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
func (c *Client) generate(ctx context.Context, model string, session *schema.Conversation, opts ...opt.Opt) (*schema.Message, error) {
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
	var response chatCompletionResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("chat", "completions")); err != nil {
		return nil, err
	}

	return c.processResponse(&response, session)
}

// generateStream handles the SSE streaming response from the Mistral API
func (c *Client) generateStream(ctx context.Context, payload client.Payload, session *schema.Conversation, streamFn opt.StreamFn) (*schema.Message, error) {
	// Accumulators for building the final response from streamed chunks
	var (
		role         string
		finishReason string
		usage        *chatUsage
		content      strings.Builder
		toolCalls    []mistralToolCall
	)

	callback := func(event client.TextStreamEvent) error {
		// Check for [DONE] sentinel
		data := strings.TrimSpace(event.Data)
		if data == "[DONE]" {
			return io.EOF
		}

		// Parse the SSE data as JSON
		var chunk chatCompletionChunk
		if err := event.Json(&chunk); err != nil {
			return err
		}

		// Extract usage (typically in the final chunk)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		// Process choices
		if len(chunk.Choices) == 0 {
			return nil
		}
		choice := chunk.Choices[0]

		// Capture finish reason
		if choice.FinishReason != "" {
			finishReason = choice.FinishReason
		}

		delta := choice.Delta

		// Capture role from first chunk
		if role == "" && delta.Role != "" {
			role = delta.Role
		}

		// Accumulate text content and stream to callback
		if delta.Content != "" {
			content.WriteString(delta.Content)
			streamFn("assistant", delta.Content)
		}

		// Accumulate tool calls
		for _, tc := range delta.ToolCalls {
			// Find existing tool call by index or id to merge partial data
			found := false
			for i := range toolCalls {
				if toolCalls[i].Id == tc.Id {
					toolCalls[i].Function.Arguments += tc.Function.Arguments
					found = true
					break
				}
			}
			if !found {
				toolCalls = append(toolCalls, tc)
			}
		}

		return nil
	}

	// Execute with streaming
	var discard chatCompletionResponse
	if err := c.DoWithContext(ctx, payload, &discard,
		client.OptPath("chat", "completions"),
		client.OptTextStreamCallback(callback),
	); err != nil {
		if err != io.EOF {
			return nil, err
		}
	}

	// Build final response from accumulated data
	msg := mistralMessage{
		Role:      role,
		Content:   content.String(),
		ToolCalls: toolCalls,
	}

	response := &chatCompletionResponse{
		Choices: []chatChoice{{
			Message:      msg,
			FinishReason: finishReason,
		}},
	}
	if usage != nil {
		response.Usage = *usage
	}

	return c.processResponse(response, session)
}

// processResponse converts a Mistral response to a schema message and appends to session
func (c *Client) processResponse(response *chatCompletionResponse, session *schema.Conversation) (*schema.Message, error) {
	// Convert response to schema message
	message, err := messageFromMistralResponse(response)
	if err != nil {
		return nil, err
	}

	// Append the message to the session with token counts
	inputTokens := uint(response.Usage.PromptTokens)
	outputTokens := uint(response.Usage.CompletionTokens)
	session.AppendWithOuput(*message, inputTokens, outputTokens)

	// Return error for finish reasons that need caller attention
	if len(response.Choices) > 0 {
		switch response.Choices[0].FinishReason {
		case finishReasonLength, finishReasonModelLength:
			return message, llm.ErrMaxTokens
		case finishReasonError:
			return message, llm.ErrInternalServerError
		}
	}

	return message, nil
}

///////////////////////////////////////////////////////////////////////////////
// REQUEST BUILDING

// generateRequestFromOpts builds a chatCompletionRequest from the session and applied options
func generateRequestFromOpts(model string, session *schema.Conversation, options opt.Options) (*chatCompletionRequest, error) {
	// Convert session to Mistral message format
	messages, err := mistralMessagesFromSession(session)
	if err != nil {
		return nil, err
	}

	request := &chatCompletionRequest{
		Model:    model,
		Messages: messages,
	}

	// System prompt — prepend as a system role message
	if systemPrompt := options.GetString(opt.SystemPromptKey); systemPrompt != "" {
		sysMsg := mistralMessage{
			Role:    roleSystem,
			Content: systemPrompt,
		}
		request.Messages = append([]mistralMessage{sysMsg}, request.Messages...)
	}

	// Temperature
	if options.Has(opt.TemperatureKey) {
		v := options.GetFloat64(opt.TemperatureKey)
		request.Temperature = &v
	}

	// Top P
	if options.Has(opt.TopPKey) {
		v := options.GetFloat64(opt.TopPKey)
		request.TopP = &v
	}

	// Max tokens
	if options.Has(opt.MaxTokensKey) {
		v := int(options.GetUint(opt.MaxTokensKey))
		request.MaxTokens = &v
	} else {
		v := defaultMaxTokens
		request.MaxTokens = &v
	}

	// Stop sequences
	if ss := options.GetStringArray(opt.StopSequencesKey); len(ss) > 0 {
		request.Stop = ss
	}

	// Random seed
	if options.Has(opt.SeedKey) {
		v := options.GetUint(opt.SeedKey)
		request.RandomSeed = &v
	}

	// Presence penalty
	if options.Has(opt.PresencePenaltyKey) {
		v := options.GetFloat64(opt.PresencePenaltyKey)
		request.PresencePenalty = &v
	}

	// Frequency penalty
	if options.Has(opt.FrequencyPenaltyKey) {
		v := options.GetFloat64(opt.FrequencyPenaltyKey)
		request.FrequencyPenalty = &v
	}

	// Safe prompt
	if options.GetBool("safe-prompt") {
		request.SafePrompt = true
	}

	// Response format (JSON schema)
	if schemaJSON := options.GetString(opt.JSONSchemaKey); schemaJSON != "" {
		request.ResponseFormat = &responseFormat{
			Type:       responseFormatJSONSchema,
			JSONSchema: json.RawMessage(schemaJSON),
		}
	}

	// Tool choice
	if tc := options.GetString(opt.ToolChoiceKey); tc != "" {
		request.ToolChoice = tc
	}

	// Tools from toolkit
	if v := options.Get(opt.ToolkitKey); v != nil {
		if tk, ok := v.(*tool.Toolkit); ok {
			tools, err := mistralToolsFromToolkit(tk)
			if err != nil {
				return nil, err
			}
			if len(tools) > 0 {
				request.Tools = tools
			}
		}
	}

	return request, nil
}

// GenerateRequest builds a generate request from options without sending it.
// Useful for testing and debugging.
func GenerateRequest(model string, session *schema.Conversation, opts ...opt.Opt) (any, error) {
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	return generateRequestFromOpts(model, session, options)
}
