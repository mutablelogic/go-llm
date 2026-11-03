package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// streamDone is the data of the event which ends a stream
const streamDone = "[DONE]"

// server is which API a client is for, as some options are sent differently
type server int

const (
	serverOther    server = iota // Another OpenAI-compatible server
	serverOpenAI                 // OpenAI's own API
	serverDeepSeek               // DeepSeek's API
	serverMistral                // Mistral's API
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutSession returns the model's reply to the messages, which must be
// for a model of this client, and the tokens used, with the Chat Completions
// API, which OpenAI-compatible servers such as DeepSeek have. The model can
// call the tools, and the reply has its calls. When the stream function isn't
// nil, the reply is streamed, and the function is called with each piece of
// its reasoning and text as it's generated. The request isn't timed out, as
// a reasoning model can take a while to reply, but is cancelled with the
// context.
func (c *Client) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if len(messages) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing messages")
	}

	// Convert the messages
	chat, err := chatMessages(ctx, messages, c.isDeepSeek() && len(tools) > 0)
	if err != nil {
		return nil, llm.Usage{}, err
	}

	// Convert the tools
	chatTools, err := chatTools(tools)
	if err != nil {
		return nil, llm.Usage{}, err
	}

	// Make the request with the messages and tools, then set the options
	request := chatRequest{
		Model:    model.Name(),
		Messages: chat,
		Tools:    chatTools,
	}
	if stream != nil {
		request.Stream, request.StreamOptions = true, &streamOptions{IncludeUsage: true}
	}
	if err := request.apply(c.server(), opts...); err != nil {
		return nil, llm.Usage{}, err
	}

	// Send the request, and receive the reply
	var response chatResponse
	var req client.Payload
	out, reqopts := any(&response), []client.RequestOpt{client.OptPath("chat", "completions"), client.OptNoTimeout()}
	var chunks *chatStream
	if stream != nil {
		chunks = &chatStream{stream: stream}
		req, err = client.NewJSONRequestEx(http.MethodPost, request, client.ContentTypeTextStream)
		out, reqopts = nil, append(reqopts, client.OptTextStreamCallback(chunks.add))
	} else {
		req, err = client.NewJSONRequest(request)
	}
	if err != nil {
		return nil, llm.Usage{}, err
	}
	if err := c.DoWithContext(ctx, req, out, reqopts...); err != nil {
		return nil, llm.Usage{}, err
	} else if chunks != nil {
		if response = chunks.Response(); !chunks.done {
			return nil, llm.Usage{}, llm.ErrServiceUnavailable.With("the reply ended before it was done")
		}
	}
	if response.Error != nil {
		return nil, llm.Usage{}, errors.New(response.Error.Message)
	} else if len(response.Choices) == 0 {
		return nil, llm.Usage{}, llm.ErrServiceUnavailable.With("the reply has no choices")
	}

	// Return the reply and usage
	return newChatResponse(response.Choices[0], response.Usage), replyUsage(response.Choices[0], response.Usage), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// server returns which API the client is for, from its host
func (c *Client) server() server {
	switch {
	case c.url == nil:
		return serverOther
	case strings.EqualFold(c.url.Hostname(), openAIHost):
		return serverOpenAI
	case strings.EqualFold(c.url.Hostname(), deepSeekHost):
		return serverDeepSeek
	case strings.EqualFold(c.url.Hostname(), mistralHost):
		return serverMistral
	default:
		return serverOther
	}
}

// isDeepSeek returns true when the client is for DeepSeek's API, which needs
// a reply's reasoning sent back with its tool calls
func (c *Client) isDeepSeek() bool {
	return c.server() == serverDeepSeek
}

// chatStream collects the chunks of a streamed reply into a response, and
// calls the stream function with the reasoning and text of each, and a
// refusal as text. The reasoning is the reasoning content, or Mistral's
// thinking parts of the content. The chunks
// have the pieces of the first choice's message, and of its tool calls, by
// their index, and the chunk with why the model stopped is done. The usage
// is in the last chunk, which has no choices, and the stream ends with
// "[DONE]". The reasoning and text are collected in builders, as there can be
// many pieces.
type chatStream struct {
	stream             llm.GeneratorStreamFn
	response           chatResponse
	choice             chatChoice
	reasoning, content strings.Builder
	refusal            strings.Builder
	calls              []chatToolCall
	arguments          []*strings.Builder
	done               bool
}

// add adds an event of the stream. A chunk with an error fails the reply.
func (s *chatStream) add(event client.TextStreamEvent) error {
	if strings.TrimSpace(event.Data) == streamDone {
		return io.EOF
	}
	var chunk chatResponse
	if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
		return err
	} else if chunk.Error != nil {
		return errors.New(chunk.Error.Message)
	}
	if chunk.Usage != nil {
		s.response.Usage = chunk.Usage
	}
	if chunk.ID != "" {
		s.response.ID, s.response.Model = chunk.ID, chunk.Model
	}

	// Add the pieces of the first choice
	for _, choice := range chunk.Choices {
		if choice.Index != 0 {
			continue
		}
		delta := choice.Delta
		text := contentText(delta.Content)
		reasoning := delta.ReasoningContent + contentThinking(delta.Content)
		s.reasoning.WriteString(reasoning)
		s.refusal.WriteString(delta.Refusal)
		s.content.WriteString(text)
		for _, call := range delta.ToolCalls {
			s.addToolCall(call)
		}
		if choice.FinishReason != "" {
			s.choice.FinishReason, s.done = choice.FinishReason, true
		}

		// Stream the reasoning and text, and a refusal as text
		if reasoning != "" {
			s.stream(llm.StreamThinking, reasoning)
		}
		if text != "" {
			s.stream(llm.StreamText, text)
		}
		if delta.Refusal != "" {
			s.stream(llm.StreamText, delta.Refusal)
		}
	}
	return nil
}

// addToolCall adds a piece of a tool call, which is the call with its index,
// or the next call when it has no index, and whose arguments are appended
func (s *chatStream) addToolCall(call chatToolCall) {
	index := len(s.calls)
	if call.Index != nil {
		index = *call.Index
	} else if call.ID == "" && index > 0 {
		index--
	}
	for index >= len(s.calls) {
		s.calls = append(s.calls, chatToolCall{Type: "function"})
		s.arguments = append(s.arguments, new(strings.Builder))
	}
	if call.ID != "" {
		s.calls[index].ID = call.ID
	}
	if call.Function.Name != "" {
		s.calls[index].Function.Name = call.Function.Name
	}
	s.arguments[index].WriteString(call.Function.Arguments)
}

// Response returns the response, with the reasoning, text and tool calls of
// the chunks as its first choice
func (s *chatStream) Response() chatResponse {
	response := s.response
	choice := s.choice
	choice.Message.Role = "assistant"
	choice.Message.ReasoningContent = s.reasoning.String()
	choice.Message.Refusal = s.refusal.String()
	choice.Message.Content = s.content.String()
	for i, call := range s.calls {
		call.Function.Arguments = s.arguments[i].String()
		choice.Message.ToolCalls = append(choice.Message.ToolCalls, call)
	}
	choice.Message.ToolCalls = slices.DeleteFunc(choice.Message.ToolCalls, func(call chatToolCall) bool {
		return call.Function.Name == ""
	})
	response.Choices = []chatChoice{choice}
	return response
}
