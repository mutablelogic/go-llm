package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession returns the model's reply to the messages, which must be
// for a model of this client, and the tokens used, with the Messages API.
// The model can call the tools, and the reply has its calls. The reply is
// always streamed, so a long one doesn't time out, and when the stream
// function isn't nil, it's called with each piece of its thinking and text
// as it's generated. The request is cached, so the next one, which starts
// the same, such as with the same system prompt and tools, is cheaper. The
// request isn't timed out, as a model can take a while to reply, but is
// cancelled with the context.
func (c *Client) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if len(messages) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing messages")
	}

	// Make the request, with the messages, the system prompt, the tools,
	// and the options
	request, err := c.newRequest(ctx, m, messages, tools, opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	request.Stream = true
	request.CacheControl = &cacheControl{Type: "ephemeral"}

	// Send the request, and collect the streamed reply
	payload, err := client.NewJSONRequestEx(http.MethodPost, request, client.ContentTypeTextStream)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	reply := &replyStream{stream: stream}
	reqopts := []client.RequestOpt{client.OptPath("messages"), client.OptNoTimeout(), client.OptTextStreamCallback(reply.add)}
	if betas := request.betas(); betas != "" {
		reqopts = append(reqopts, client.OptReqHeader("anthropic-beta", betas))
	}
	if err := c.DoWithContext(ctx, payload, nil, reqopts...); err != nil {
		return nil, llm.Usage{}, err
	} else if !reply.done {
		return nil, llm.Usage{}, llm.ErrServiceUnavailable.With("the reply ended before it was done")
	}

	// Return the reply and the tokens used
	response := reply.response()
	return newReply(response), replyUsage(response.Usage), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// newRequest returns a request to the model, with the messages, the system
// prompt, which is the system messages, the tools, and the options
func (c *Client) newRequest(ctx context.Context, model *Model, messages []llm.Message, tools []llm.Tool, opts ...llm.Opt) (*messagesRequest, error) {
	wire, system, err := newMessages(ctx, messages)
	if err != nil {
		return nil, err
	}
	wireTools, err := newTools(tools)
	if err != nil {
		return nil, err
	}
	request := &messagesRequest{Model: model.Name(), System: system, Messages: wire, Tools: wireTools}
	if err := request.apply(model, opts...); err != nil {
		return nil, err
	}
	return request, nil
}

// newTools returns the tools as the API's, with each tool's input schema,
// or an object without properties, for a tool without one, or nil, when
// there are none. Returns an error if a tool is nil, or its name isn't valid
// or unique.
func newTools(tools []llm.Tool) ([]tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	result := make([]tool, 0, len(tools))
	names := make(map[string]bool, len(tools))
	for _, t := range tools {
		if t == nil {
			return nil, llm.ErrBadParameter.With("missing tool")
		} else if name := t.Name(); !llm.IsToolName(name) {
			return nil, llm.ErrBadParameter.Withf("invalid tool name %q", name)
		} else if names[name] {
			return nil, llm.ErrBadParameter.Withf("duplicate tool %q", name)
		} else {
			names[name] = true
		}
		schema := json.RawMessage(`{"type":"object","properties":{}}`)
		if s := t.InputSchema(); s != nil {
			data, err := json.Marshal(s)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("tool %q: %v", t.Name(), err)
			}
			schema = data
		}
		result = append(result, tool{Name: t.Name(), Description: t.Description(), InputSchema: schema})
	}
	return result, nil
}

// replyUsage returns the tokens used: the input tokens, which are those of
// the request, including those read from, and written to, the cache, of
// which those read from it are cached, and the output tokens
func replyUsage(u usage) llm.Usage {
	return llm.Usage{
		InputTokens:  u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
		OutputTokens: u.OutputTokens,
		CachedTokens: u.CacheReadInputTokens,
	}
}

///////////////////////////////////////////////////////////////////////////////
// STREAM

// replyStream collects the events of a streamed reply into a response, and
// calls the stream function, when there is one, with each piece of its
// thinking and text. The reply starts with the message, then each content
// block starts, has its pieces, and stops, and then the message has why the
// model stopped, and its output tokens, and stops.
type replyStream struct {
	stream  llm.GeneratorStreamFn
	message messagesResponse
	blocks  []*streamBlock
	done    bool
}

// streamBlock is a content block, as its pieces are streamed
type streamBlock struct {
	block               contentBlock
	text, thinking, arg strings.Builder
}

// add adds an event of the stream. An error event fails the reply, and the
// stream ends when the message stops.
func (s *replyStream) add(event client.TextStreamEvent) error {
	if strings.TrimSpace(event.Data) == "" {
		return nil
	}
	var e streamEvent
	if err := json.Unmarshal([]byte(event.Data), &e); err != nil {
		return err
	}
	switch e.Type {
	case "message_start":
		if e.Message != nil {
			s.message = *e.Message
		}
	case "content_block_start":
		if e.ContentBlock != nil {
			for e.Index >= len(s.blocks) {
				s.blocks = append(s.blocks, &streamBlock{})
			}
			s.blocks[e.Index].block = *e.ContentBlock
		}
	case "content_block_delta":
		if e.Delta != nil && e.Index < len(s.blocks) {
			s.addDelta(s.blocks[e.Index], *e.Delta)
		}
	case "message_delta":
		if e.Delta != nil && e.Delta.StopReason != "" {
			s.message.StopReason = e.Delta.StopReason
		}
		if e.Usage != nil {
			s.message.Usage = mergeUsage(s.message.Usage, *e.Usage)
		}
	case "message_stop":
		s.done = true
		return io.EOF
	case "error":
		if e.Error != nil {
			return streamError(*e.Error)
		}
		return llm.ErrServiceUnavailable.With("the reply failed")
	}
	return nil
}

// addDelta adds a piece of a content block, and streams its thinking, or
// text
func (s *replyStream) addDelta(block *streamBlock, delta streamDelta) {
	switch delta.Type {
	case "text_delta":
		block.text.WriteString(delta.Text)
		if s.stream != nil && delta.Text != "" {
			s.stream(llm.StreamText, delta.Text)
		}
	case "thinking_delta":
		block.thinking.WriteString(delta.Thinking)
		if s.stream != nil && delta.Thinking != "" {
			s.stream(llm.StreamThinking, delta.Thinking)
		}
	case "signature_delta":
		block.block.Signature += delta.Signature
	case "input_json_delta":
		block.arg.WriteString(delta.PartialJSON)
	}
}

// response returns the response, with the content blocks which were
// streamed
func (s *replyStream) response() messagesResponse {
	response := s.message
	response.Content = make([]contentBlock, 0, len(s.blocks))
	for _, b := range s.blocks {
		block := b.block
		switch block.Type {
		case "text":
			block.Text += b.text.String()
		case "thinking":
			block.Thinking = types.Ptr(types.Value(block.Thinking) + b.thinking.String())
		case "tool_use":
			if arg := strings.TrimSpace(b.arg.String()); arg != "" {
				block.Input = json.RawMessage(arg)
			}
		}
		response.Content = append(response.Content, block)
	}
	return response
}

// mergeUsage returns the usage, with the tokens of the other, which are
// those reported as the message ends, where they're reported
func mergeUsage(u, other usage) usage {
	if other.InputTokens > 0 {
		u.InputTokens = other.InputTokens
	}
	if other.OutputTokens > 0 {
		u.OutputTokens = other.OutputTokens
	}
	if other.CacheReadInputTokens > 0 {
		u.CacheReadInputTokens = other.CacheReadInputTokens
	}
	if other.CacheCreationInputTokens > 0 {
		u.CacheCreationInputTokens = other.CacheCreationInputTokens
	}
	return u
}

// streamError returns the error of a stream, as the error it's like, such as
// that the API is overloaded, which is unavailable, so it's tried again
func streamError(e apiError) error {
	switch e.Type {
	case "overloaded_error", "api_error":
		return llm.ErrServiceUnavailable.With(e.Message)
	case "rate_limit_error":
		return llm.ErrServiceUnavailable.Withf("rate limited: %s", e.Message)
	case "invalid_request_error":
		return llm.ErrBadParameter.With(e.Message)
	case "authentication_error", "permission_error":
		return llm.ErrForbidden.With(e.Message)
	case "not_found_error":
		return llm.ErrNotFound.With(e.Message)
	default:
		return llm.ErrInternalServerError.Withf("%s: %s", e.Type, e.Message)
	}
}
