package google

import (
	"context"
	"encoding/json"
	"io"

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
func (c *Client) WithoutSession(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, *schema.Usage, error) {
	if message == nil {
		return nil, nil, llm.ErrBadParameter.With("message is required")
	}
	session := schema.Conversation{message}
	return c.generate(ctx, model.Name, &session, opts...)
}

// WithSession sends a message within a session and returns the response (stateful)
func (c *Client) WithSession(ctx context.Context, model schema.Model, session *schema.Conversation, message *schema.Message, opts ...opt.Opt) (*schema.Message, *schema.Usage, error) {
	if session == nil {
		return nil, nil, llm.ErrBadParameter.With("session is required")
	}
	if message == nil {
		return nil, nil, llm.ErrBadParameter.With("message is required")
	}
	session.Append(*message)
	return c.generate(ctx, model.Name, session, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// generate is the core method that builds a request from options and sends it
func (c *Client) generate(ctx context.Context, model string, session *schema.Conversation, opts ...opt.Opt) (*schema.Message, *schema.Usage, error) {
	// Apply options
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, nil, err
	}
	streamFn := options.GetStream()

	// Build request
	request, err := generateRequestFromOpts(model, session, options)
	if err != nil {
		return nil, nil, err
	}

	// Create JSON payload
	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, nil, err
	}

	// Streaming path
	if streamFn != nil {
		return c.generateStream(ctx, model, payload, session, streamFn)
	}

	// Non-streaming path
	var response geminiGenerateResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("models", model+":generateContent")); err != nil {
		return nil, nil, err
	}

	return c.processResponse(&response, session)
}

// generateStream handles the SSE streaming response from the Gemini API
func (c *Client) generateStream(ctx context.Context, model string, payload client.Payload, session *schema.Conversation, streamFn opt.StreamFn) (*schema.Message, *schema.Usage, error) {
	// Accumulators for building the final response from streamed chunks
	var (
		role        string
		finishReson string
		usage       *geminiUsageMetadata
		allParts    []*geminiPart
	)

	callback := func(event client.TextStreamEvent) error {
		var chunk geminiGenerateResponse
		if err := event.Json(&chunk); err != nil {
			return err
		}

		// Extract usage metadata (last chunk typically has the final counts)
		if chunk.UsageMetadata != nil {
			usage = chunk.UsageMetadata
		}

		// Process candidates
		if len(chunk.Candidates) == 0 {
			return nil
		}
		candidate := chunk.Candidates[0]

		// Capture finish reason
		if candidate.FinishReason != "" {
			finishReson = candidate.FinishReason
		}

		if candidate.Content == nil {
			return nil
		}

		// Capture role from first chunk
		if role == "" && candidate.Content.Role != "" {
			role = candidate.Content.Role
		}

		// Accumulate parts and stream text/thinking to callback
		for _, part := range candidate.Content.Parts {
			allParts = append(allParts, part)

			if part.Text != "" {
				if part.Thought {
					streamFn("thinking", part.Text)
				} else {
					streamFn("assistant", part.Text)
				}
			}
		}

		return nil
	}

	// Execute with SSE streaming (Gemini uses ?alt=sse)
	var discard geminiGenerateResponse
	if err := c.DoWithContext(ctx, payload, &discard,
		client.OptPath("models", model+":streamGenerateContent"),
		client.OptQuery(map[string][]string{"alt": {"sse"}}),
		client.OptTextStreamCallback(callback),
	); err != nil {
		// io.EOF signals normal end of stream
		if err != io.EOF {
			return nil, nil, err
		}
	}

	// Build final response from accumulated parts
	response := &geminiGenerateResponse{
		Candidates: []*geminiCandidate{{
			Content: &geminiContent{
				Parts: allParts,
				Role:  role,
			},
			FinishReason: finishReson,
		}},
		UsageMetadata: usage,
	}

	return c.processResponse(response, session)
}

// processResponse converts a gemini response to a schema message and appends to session
func (c *Client) processResponse(response *geminiGenerateResponse, session *schema.Conversation) (*schema.Message, *schema.Usage, error) {
	message, err := messageFromGeminiResponse(response)
	if err != nil {
		return nil, nil, err
	}

	// Append the message to the session with token counts
	var inputTokens, outputTokens uint
	if response.UsageMetadata != nil {
		inputTokens = uint(response.UsageMetadata.PromptTokenCount)
		outputTokens = uint(response.UsageMetadata.CandidatesTokenCount)
	}
	session.AppendWithOuput(*message, inputTokens, outputTokens)

	// Build usage
	usageResult := &schema.Usage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}

	// Return error for finish reasons that need caller attention
	if len(response.Candidates) > 0 {
		switch response.Candidates[0].FinishReason {
		case geminiFinishReasonMaxTokens:
			return message, usageResult, llm.ErrMaxTokens
		case geminiFinishReasonSafety, geminiFinishReasonImageSafety:
			return message, usageResult, llm.ErrRefusal
		}
	}

	return message, usageResult, nil
}

///////////////////////////////////////////////////////////////////////////////
// REQUEST BUILDING

// generateRequestFromOpts builds a geminiGenerateRequest from the session and applied options
func generateRequestFromOpts(model string, session *schema.Conversation, options opt.Options) (*geminiGenerateRequest, error) {
	// Convert session messages to wire contents
	contents, err := geminiContentsFromSession(session)
	if err != nil {
		return nil, err
	}

	request := &geminiGenerateRequest{
		Contents: contents,
	}

	// System instruction
	if systemPrompt := options.GetString(opt.SystemPromptKey); systemPrompt != "" {
		request.SystemInstruction = geminiNewTextContent("", systemPrompt)
	}

	// Generation config — fields are set directly; the omitzero tag on the
	// struct ensures the whole block is omitted when nothing is configured.
	if options.Has(opt.TemperatureKey) {
		v := options.GetFloat64(opt.TemperatureKey)
		request.GenerationConfig.Temperature = &v
	}
	if options.Has(opt.MaxTokensKey) {
		request.GenerationConfig.MaxOutputTokens = int(options.GetUint(opt.MaxTokensKey))
	}
	if options.Has(opt.TopKKey) {
		v := int(options.GetUint(opt.TopKKey))
		request.GenerationConfig.TopK = &v
	}
	if options.Has(opt.TopPKey) {
		v := options.GetFloat64(opt.TopPKey)
		request.GenerationConfig.TopP = &v
	}
	if ss := options.GetStringArray(opt.StopSequencesKey); len(ss) > 0 {
		request.GenerationConfig.StopSequences = ss
	}
	if options.GetBool(opt.ThinkingKey) || options.Has(opt.ThinkingBudgetKey) {
		request.GenerationConfig.ThinkingConfig = &geminiThinkingConfig{
			IncludeThoughts: true,
		}
		if options.Has(opt.ThinkingBudgetKey) {
			request.GenerationConfig.ThinkingConfig.ThinkingBudget = int(options.GetUint(opt.ThinkingBudgetKey))
		}
	}
	if v := options.Get(opt.SeedKey); v != nil {
		if seed, ok := v.(int); ok {
			request.GenerationConfig.Seed = &seed
		}
	}
	if options.Has(opt.PresencePenaltyKey) {
		v := options.GetFloat64(opt.PresencePenaltyKey)
		request.GenerationConfig.PresencePenalty = &v
	}
	if options.Has(opt.FrequencyPenaltyKey) {
		v := options.GetFloat64(opt.FrequencyPenaltyKey)
		request.GenerationConfig.FrequencyPenalty = &v
	}
	if schemaJSON := options.GetString(opt.JSONSchemaKey); schemaJSON != "" {
		var s any
		if err := json.Unmarshal([]byte(schemaJSON), &s); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid JSON schema: %v", err)
		}
		request.GenerationConfig.ResponseMIMEType = "application/json"
		request.GenerationConfig.ResponseJSONSchema = s
	}

	// Tools from toolkit
	if v := options.Get(opt.ToolkitKey); v != nil {
		if tk, ok := v.(*tool.Toolkit); ok {
			decls := geminiFunctionDeclsFromToolkit(tk)
			if len(decls) > 0 {
				request.Tools = []*geminiTool{{
					FunctionDeclarations: decls,
				}}
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
