package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession returns the model's reply to the messages, which must be
// for a model of this client, and the tokens used, with
// streamGenerateContent. The model can call the tools, and the reply has
// its calls, with their thought signatures, which are sent back with them.
// The reply is always streamed, so a long one doesn't time out, and when
// the stream function isn't nil, it's called with each piece of its
// thinking, which is a summary of the model's thoughts, and its text, as
// they're generated. The request isn't timed out, as a model can take a
// while to reply, but is cancelled with the context.
func (c *Client) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if len(messages) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing messages")
	}

	// Make the request, with the messages, the system prompt, the tools,
	// and the options
	request, err := newRequest(ctx, m, messages, tools, opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}

	// Send the request, and collect the streamed reply
	payload, err := client.NewJSONRequestEx(http.MethodPost, request, client.ContentTypeTextStream)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	reply := &replyStream{stream: stream}
	if err := c.DoWithContext(ctx, payload, nil,
		client.OptPath("models", m.Name()+":streamGenerateContent"),
		client.OptQuery(url.Values{"alt": {"sse"}}),
		client.OptNoTimeout(),
		client.OptTextStreamCallback(reply.add),
	); err != nil {
		return nil, llm.Usage{}, err
	} else if reply.err != nil {
		return nil, llm.Usage{}, reply.err
	} else if !reply.done {
		return nil, llm.Usage{}, llm.ErrServiceUnavailable.With("the reply ended before it was done")
	}

	// Return the reply and the tokens used
	return newReply(reply.parts, reply.finish, reply.blocked, reply.usage), replyUsage(reply.usage), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newRequest returns a request to the model, with the messages, the system
// prompt, which is the system messages, the tools, and the options
func newRequest(ctx context.Context, model *Model, messages []llm.Message, tools []llm.Tool, opts ...llm.Opt) (*generateRequest, error) {
	contents, system, err := newContents(ctx, model, messages)
	if err != nil {
		return nil, err
	}
	request := &generateRequest{Contents: contents}
	if system != "" {
		request.SystemInstruction = &content{Parts: []part{{Text: system}}}
	}
	if request.Tools, err = newTools(tools); err != nil {
		return nil, err
	}
	if err := request.apply(model, opts...); err != nil {
		return nil, err
	}
	return request, nil
}

// newTools returns the tools as the API's functions, with each tool's input
// schema, or an object without properties, for a tool without one, or nil,
// when there are none. Returns an error if a tool is nil, or its name isn't
// valid or unique.
func newTools(tools []llm.Tool) ([]tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	functions := make([]functionDeclaration, 0, len(tools))
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
		functions = append(functions, functionDeclaration{Name: t.Name(), Description: t.Description(), ParametersJSONSchema: schema})
	}
	return []tool{{FunctionDeclarations: functions}}, nil
}

// replyUsage returns the tokens used: the input tokens, which are those of
// the prompt, and of the tools' prompts, of which those read from the cache
// are cached, and the output tokens, which are those of the reply, and of
// its thoughts, which are its reasoning
func replyUsage(u usageMetadata) llm.Usage {
	return llm.Usage{
		InputTokens:     u.PromptTokenCount + u.ToolUsePromptTokenCount,
		OutputTokens:    u.CandidatesTokenCount + u.ThoughtsTokenCount,
		CachedTokens:    u.CachedContentTokenCount,
		ReasoningTokens: u.ThoughtsTokenCount,
	}
}

///////////////////////////////////////////////////////////////////////////////
// STREAM

// replyStream collects the pieces of a streamed reply, each of which is a
// response with the next parts of its candidate, and calls the stream
// function, when there is one, with each piece of its thinking and text.
// The last piece has why the model finished, and the tokens used.
type replyStream struct {
	stream  llm.GeneratorStreamFn
	parts   []part
	finish  string
	blocked string
	usage   usageMetadata
	done    bool
	err     error
}

// add adds a piece of the reply. An error fails the reply, and the reply is
// done once the model finishes, or the prompt is blocked.
func (s *replyStream) add(event client.TextStreamEvent) error {
	if strings.TrimSpace(event.Data) == "" {
		return nil
	}
	var response generateResponse
	if err := json.Unmarshal([]byte(event.Data), &response); err != nil {
		return err
	}
	if response.Error != nil {
		s.err = streamError(*response.Error)
		return s.err
	}
	if response.UsageMetadata != nil {
		s.usage = *response.UsageMetadata
	}
	if response.PromptFeedback != nil && response.PromptFeedback.BlockReason != "" {
		s.blocked, s.done = response.PromptFeedback.BlockReason, true
	}
	for _, candidate := range response.Candidates {
		if candidate.Content != nil {
			for _, p := range candidate.Content.Parts {
				s.addPart(p)
			}
		}
		if candidate.FinishReason != "" {
			s.finish, s.done = candidate.FinishReason, true
		}
	}
	return nil
}

// addPart adds a part of the reply, and streams its thinking, or text. A
// signature which comes in a part of its own, without a call, thinking or
// text, is the signature of the part before it, such as a call, which is
// sent back with it.
func (s *replyStream) addPart(p part) {
	switch {
	case p.FunctionCall == nil && !p.Thought && p.Text == "" && p.ThoughtSignature != "":
		if n := len(s.parts); n > 0 && s.parts[n-1].ThoughtSignature == "" {
			s.parts[n-1].ThoughtSignature = p.ThoughtSignature
		}
	case p.FunctionCall != nil:
		s.parts = append(s.parts, p)
	case p.Thought:
		if s.stream != nil && p.Text != "" {
			s.stream(llm.StreamThinking, p.Text)
		}
		s.parts = append(s.parts, p)
	case p.Text != "":
		if s.stream != nil {
			s.stream(llm.StreamText, p.Text)
		}
		s.parts = append(s.parts, p)
	}
}

// streamError returns the error the API returned, as the error it's like,
// such as that it's rate limited, which is unavailable, so it's tried again
func streamError(e apiError) error {
	switch e.Status {
	case "RESOURCE_EXHAUSTED":
		return llm.ErrServiceUnavailable.Withf("rate limited: %s", e.Message)
	case "UNAVAILABLE", "INTERNAL", "DEADLINE_EXCEEDED":
		return llm.ErrServiceUnavailable.With(e.Message)
	case "INVALID_ARGUMENT", "FAILED_PRECONDITION", "OUT_OF_RANGE":
		return llm.ErrBadParameter.With(e.Message)
	case "PERMISSION_DENIED", "UNAUTHENTICATED":
		return llm.ErrForbidden.With(e.Message)
	case "NOT_FOUND":
		return llm.ErrNotFound.With(e.Message)
	default:
		return llm.ErrInternalServerError.Withf("%s: %s", e.Status, e.Message)
	}
}
