package ollama

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Generator = (*Client)(nil)

type chatStreamAccumulator struct {
	role      string
	toolCalls []chatToolCall
	content   strings.Builder
	thinking  strings.Builder
}

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

	callback := func(v json.RawMessage) error {
		var chunk generateResponse
		if err := json.Unmarshal(v, &chunk); err != nil {
			return err
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
			final = chunk
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
	var acc chatStreamAccumulator

	callback := func(v json.RawMessage) error {
		var chunk chatResponse
		if err := json.Unmarshal(v, &chunk); err != nil {
			return err
		}
		acc.consume(&chunk, streamFn)
		if chunk.Done {
			final = chunk
		}
		return nil
	}

	var discard chatResponse
	if err := c.DoWithContext(ctx, payload, &discard, client.OptPath("chat"), client.OptJsonStreamCallback(callback)); err != nil {
		return nil, nil, err
	}

	// Populate the final message with any state streamed in earlier chunks.
	// Ollama may emit tool calls before the terminal done=true frame, and that
	// final frame can still report done_reason=stop with an empty message body.
	acc.apply(&final)

	return c.processChatResponse(session, &final)
}

func (a *chatStreamAccumulator) consume(chunk *chatResponse, streamFn opt.StreamFn) {
	if chunk == nil {
		return
	}
	if chunk.Message.Role != "" {
		a.role = chunk.Message.Role
	}
	if content := chunk.Message.Content; content != "" {
		a.content.WriteString(content)
		if streamFn != nil {
			streamFn(schema.RoleAssistant, content)
		}
	}
	if thinking := chunk.Message.Thinking; thinking != "" {
		a.thinking.WriteString(thinking)
		if streamFn != nil {
			streamFn(schema.RoleThinking, thinking)
		}
	}
	if len(chunk.Message.ToolCalls) > 0 {
		a.toolCalls = cloneChatToolCalls(chunk.Message.ToolCalls)
	}
}

func (a *chatStreamAccumulator) apply(final *chatResponse) {
	if final == nil {
		return
	}
	if a.role != "" {
		final.Message.Role = a.role
	}
	if final.Message.Role == "" {
		final.Message.Role = schema.RoleAssistant
	}
	final.Message.Content = a.content.String()
	final.Message.Thinking = a.thinking.String()
	if len(a.toolCalls) > 0 {
		final.Message.ToolCalls = cloneChatToolCalls(a.toolCalls)
	}
}

func cloneChatToolCalls(src []chatToolCall) []chatToolCall {
	if len(src) == 0 {
		return nil
	}
	dst := make([]chatToolCall, len(src))
	copy(dst, src)
	return dst
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
