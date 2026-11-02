package ollama

import (
	"context"
	"encoding/json"
	"errors"
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
// for an Ollama model, and the tokens used. The model can call the tools, and
// the reply has its calls. When the stream function isn't nil, the reply is
// streamed, and the function is called with each piece of its thinking and
// text as it's generated. The request isn't timed out, as a model can take a
// while to load and reply, but is cancelled with the context.
func (c *Client) WithoutSession(ctx context.Context, model llm.Model, messages []llm.Message, tools []llm.Tool, stream llm.GeneratorStreamFn, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.Ollama {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not an ollama model")
	} else if len(messages) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing messages")
	}

	// Convert the messages
	chat, err := chatMessages(ctx, messages)
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
		Stream:   types.Ptr(stream != nil),
	}
	if err := request.apply(opts...); err != nil {
		return nil, llm.Usage{}, err
	}

	// Send the request, and receive the reply
	req, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var response chatResponse
	var chunks *chatStream
	out, reqopts := any(&response), []client.RequestOpt{client.OptPath("chat"), client.OptNoTimeout()}
	if stream != nil {
		chunks = &chatStream{stream: stream}
		out, reqopts = nil, append(reqopts, client.OptJsonStreamCallback(chunks.add))
	}
	if err := c.DoWithContext(ctx, req, out, reqopts...); err != nil {
		// Ollama removes messages from the start of a prompt which is more
		// than its context window, and fails when the user's message is
		// removed
		if strings.Contains(err.Error(), "no user query found in messages") {
			return nil, llm.Usage{}, llm.ErrBadParameter.Withf("the prompt is more than the context window: %v", err)
		}
		return nil, llm.Usage{}, err
	} else if chunks != nil {
		if response = chunks.Response(); !response.Done {
			return nil, llm.Usage{}, llm.ErrServiceUnavailable.With("the reply ended before it was done")
		}
	}

	// Return the reply and usage
	reply := newChatResponse(response)
	return reply, llm.Usage{
		InputTokens:  uint(max(response.PromptEvalCount, 0)),
		OutputTokens: uint(max(response.EvalCount, 0)),
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// chatStream collects the chunks of a streamed reply into a response, and
// calls the stream function with the thinking and text of each. The chunks
// have the pieces of the message, and any tool calls, and the last chunk is
// done, with why the model stopped and the tokens used. The thinking and text
// are collected in builders, as there can be many pieces.
type chatStream struct {
	stream            llm.GeneratorStreamFn
	response          chatResponse
	thinking, content strings.Builder
}

// add adds a chunk of the reply. A chunk with an error fails the reply.
func (s *chatStream) add(data json.RawMessage) error {
	var chunk struct {
		chatResponse
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(data, &chunk); err != nil {
		return err
	} else if chunk.Error != "" {
		return errors.New(chunk.Error)
	}

	// Add the pieces of the message
	message := chunk.Message
	if message.Role != "" {
		s.response.Message.Role = message.Role
	}
	s.thinking.WriteString(message.Thinking)
	s.content.WriteString(message.Content)
	s.response.Message.Images = append(s.response.Message.Images, message.Images...)
	s.response.Message.ToolCalls = append(s.response.Message.ToolCalls, message.ToolCalls...)

	// The last chunk has the model, why it stopped, and the tokens used
	if chunk.Done {
		s.response.Model = chunk.Model
		s.response.CreatedAt = chunk.CreatedAt
		s.response.Done = true
		s.response.DoneReason = chunk.DoneReason
		s.response.chatMetrics = chunk.chatMetrics
	}

	// Stream the thinking and text
	if message.Thinking != "" {
		s.stream(llm.StreamThinking, message.Thinking)
	}
	if message.Content != "" {
		s.stream(llm.StreamText, message.Content)
	}
	return nil
}

// Response returns the response, with the thinking and text of the chunks
func (s *chatStream) Response() chatResponse {
	response := s.response
	response.Message.Thinking = s.thinking.String()
	response.Message.Content = s.content.String()
	return response
}
