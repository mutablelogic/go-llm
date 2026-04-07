package ollama

import (
	"context"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession sends a single stateless message via POST /api/generate and
// returns the response. It does not support tools, thinking, or tool_choice.
func (c *Client) WithoutSession(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, *schema.UsageMeta, error) {
	if message == nil {
		return nil, nil, schema.ErrBadParameter.With("message is required")
	}
	return c.generate(ctx, model.Name, message, opts...)
}

// WithSession sends a message within a multi-turn conversation via POST
// /api/chat and returns the response. The message is appended to the session
// before the request is sent; the assistant reply is appended afterwards.
func (c *Client) WithSession(ctx context.Context, model schema.Model, session *schema.Conversation, message *schema.Message, opts ...opt.Opt) (*schema.Message, *schema.UsageMeta, error) {
	if session == nil {
		return nil, nil, schema.ErrBadParameter.With("session is required")
	}
	if message == nil {
		return nil, nil, schema.ErrBadParameter.With("message is required")
	}
	session.Append(*message)
	return c.chat(ctx, model.Name, session, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS — /api/generate

func (c *Client) generate(ctx context.Context, model string, message *schema.Message, opts ...opt.Opt) (*schema.Message, *schema.UsageMeta, error) {
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, nil, err
	}
	streamFn := options.GetStream()

	request, err := generateRequestFromOpts(model, message, options)
	if err != nil {
		return nil, nil, err
	}

	stream := streamFn != nil
	request.Stream = &stream

	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, nil, err
	}

	if streamFn != nil {
		return c.generateStream(ctx, payload, streamFn)
	}

	var response generateResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("generate")); err != nil {
		return nil, nil, err
	}

	return c.processGenerateResponse(&response)
}

// generateStream handles ndjson streaming for /api/generate.
// Ollama sends one JSON object per line; the final object has done=true.
func (c *Client) generateStream(ctx context.Context, payload client.Payload, streamFn opt.StreamFn) (*schema.Message, *schema.UsageMeta, error) {
	var final generateResponse
	var accResponse strings.Builder

	callback := func(v any) error {
		chunk, ok := v.(*generateResponse)
		if !ok || chunk == nil {
			return nil
		}
		// Read and immediately reset before the next json.Decode call.
		// json.Decode only sets fields present in the JSON; omitempty fields
		// are NOT zeroed between chunks, so stale values would leak.
		response := chunk.Response
		chunk.Response = ""

		if response != "" {
			accResponse.WriteString(response)
			streamFn("assistant", response)
		}
		if chunk.Done {
			final = *chunk
		}
		return nil
	}

	var discard generateResponse
	if err := c.DoWithContext(ctx, payload, &discard, client.OptPath("generate"), client.OptJsonStreamCallback(callback)); err != nil {
		return nil, nil, err
	}

	// The done=true chunk always has an empty Response; restore the full
	// accumulated text before passing to processGenerateResponse.
	final.Response = accResponse.String()

	return c.processGenerateResponse(&final)
}

func (c *Client) processGenerateResponse(resp *generateResponse) (*schema.Message, *schema.UsageMeta, error) {
	message, err := messageFromGenerateResponse(resp)
	if err != nil {
		return nil, nil, err
	}
	usage := &schema.UsageMeta{
		InputTokens:  uint(resp.PromptEvalCount),
		OutputTokens: uint(resp.EvalCount),
	}
	if resp.DoneReason == "length" {
		return message, usage, schema.ErrMaxTokens
	}
	return message, usage, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS — /api/chat

func (c *Client) chat(ctx context.Context, model string, session *schema.Conversation, opts ...opt.Opt) (*schema.Message, *schema.UsageMeta, error) {
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, nil, err
	}
	streamFn := options.GetStream()

	request, err := chatRequestFromOpts(model, session, options)
	if err != nil {
		return nil, nil, err
	}

	stream := streamFn != nil
	request.Stream = &stream

	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, nil, err
	}

	if streamFn != nil {
		return c.chatStream(ctx, payload, session, streamFn)
	}

	var response chatResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("chat")); err != nil {
		return nil, nil, err
	}

	return c.processChatResponse(session, &response)
}

// chatStream handles ndjson streaming for /api/chat.
func (c *Client) chatStream(ctx context.Context, payload client.Payload, session *schema.Conversation, streamFn opt.StreamFn) (*schema.Message, *schema.UsageMeta, error) {
	var final chatResponse
	var accContent, accThinking strings.Builder

	callback := func(v any) error {
		chunk, ok := v.(*chatResponse)
		if !ok || chunk == nil {
			return nil
		}
		// Read and immediately reset before the next json.Decode call.
		// json.Decode only sets fields present in the JSON; omitempty fields
		// (like "thinking") are NOT zeroed between chunks, so stale values
		// would leak into subsequent content-only chunks.
		content := chunk.Message.Content
		thinking := chunk.Message.Thinking
		chunk.Message.Content = ""
		chunk.Message.Thinking = ""

		if content != "" {
			accContent.WriteString(content)
			streamFn("assistant", content)
		}
		if thinking != "" {
			accThinking.WriteString(thinking)
			streamFn("thinking", thinking)
		}
		if chunk.Done {
			final = *chunk
		}
		return nil
	}

	var discard chatResponse
	if err := c.DoWithContext(ctx, payload, &discard, client.OptPath("chat"), client.OptJsonStreamCallback(callback)); err != nil {
		return nil, nil, err
	}

	// Populate the final message with accumulated content from streaming chunks.
	// The done=true chunk always has empty message content, so we restore the
	// full text here before passing to processChatResponse.
	final.Message.Role = "assistant"
	final.Message.Content = accContent.String()
	final.Message.Thinking = accThinking.String()

	return c.processChatResponse(session, &final)
}

func (c *Client) processChatResponse(session *schema.Conversation, resp *chatResponse) (*schema.Message, *schema.UsageMeta, error) {
	message, err := messageFromOllamaResponse(resp)
	if err != nil {
		return nil, nil, err
	}
	usage := &schema.UsageMeta{
		InputTokens:  uint(resp.PromptEvalCount),
		OutputTokens: uint(resp.EvalCount),
	}
	session.AppendWithOuput(*message, usage.InputTokens, usage.OutputTokens)
	if resp.DoneReason == "length" {
		return message, usage, schema.ErrMaxTokens
	}
	return message, usage, nil
}
