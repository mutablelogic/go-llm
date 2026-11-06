package ollama

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newChatResponse returns the message in a response from POST /api/chat, with
// its thinking, text, images and tool calls as content blocks, and why the
// model stopped
func newChatResponse(response chatResponse) *schema.Message {
	chat := response.Message
	message := &schema.Message{
		Sender:  chat.Role,
		Content: []schema.ContentBlock{},
		Tokens:  uint64(max(response.EvalCount, 0)),
		Result:  chatResult(response),
	}
	if chat.Thinking != "" {
		message.Content = append(message.Content, schema.ContentBlock{Thinking: types.Ptr(chat.Thinking)})
	}
	if chat.Content != "" {
		message.Content = append(message.Content, schema.ContentBlock{Text: types.Ptr(chat.Content)})
	}
	for _, data := range chat.Images {
		message.Content = append(message.Content, schema.ContentBlock{Resource: &schema.Resource{
			ResourceMeta: schema.ResourceMeta{
				ContentType: http.DetectContentType(data),
				Data:        data,
			},
		}})
	}
	for _, call := range chat.ToolCalls {
		message.Content = append(message.Content, schema.ContentBlock{ToolCall: &schema.ToolCall{
			ID:    call.ID,
			Name:  call.Function.Name,
			Input: call.Function.Arguments,
		}})
	}
	return message
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// chatMessages converts messages to the wire format for POST /api/chat. A
// message with more than one tool result is more than one tool message.
func chatMessages(ctx context.Context, messages []llm.Message) ([]chatMessage, error) {
	result := make([]chatMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			return nil, llm.ErrBadParameter.With("missing message")
		}
		chat, err := newChatMessages(ctx, message)
		if err != nil {
			return nil, err
		}
		result = append(result, chat...)
	}
	return result, nil
}

// newChatMessages converts a message to the wire format for POST /api/chat.
// Thinking is sent as an assistant message's thinking, and images are read
// and sent inline, as Ollama only accepts images. An assistant message's tool
// calls are sent with it, and each tool result is a tool message, with the
// tool's name.
func newChatMessages(ctx context.Context, message llm.Message) ([]chatMessage, error) {
	var chat chatMessage
	switch role := message.Role(); role {
	case "user", "assistant", "system":
		chat.Role, chat.Content = role, message.Text()
	case "thinking":
		chat.Role, chat.Thinking = "assistant", message.Text()
	case "tool":
		return toolMessages(message)
	default:
		return nil, llm.ErrBadParameter.Withf("unsupported message role %q", role)
	}
	for _, resource := range message.Resources() {
		if !isImage(resource.Type()) {
			return nil, llm.ErrBadParameter.Withf("unsupported resource type %q", resource.Type())
		}
		data, err := llm.ReadAll(ctx, resource)
		if err != nil {
			return nil, err
		}
		chat.Images = append(chat.Images, data)
	}
	for _, call := range message.ToolCalls() {
		if chat.Role != "assistant" {
			return nil, llm.ErrBadParameter.Withf("tool calls can't be in a %q message", chat.Role)
		}
		arguments := call.Input
		if len(arguments) == 0 {
			arguments = json.RawMessage(`{}`)
		}
		chat.ToolCalls = append(chat.ToolCalls, chatToolCall{
			ID:       call.ID,
			Function: chatToolCallFunction{Name: call.Name, Arguments: arguments},
		})
	}
	return []chatMessage{chat}, nil
}

// toolMessages returns a tool message for each tool result in the message,
// with the result as its content, or the message's text when it has no
// results. An error is sent as its result.
func toolMessages(message llm.Message) ([]chatMessage, error) {
	results := message.ToolResults()
	if len(results) == 0 {
		return []chatMessage{{Role: "tool", Content: message.Text()}}, nil
	}
	chat := make([]chatMessage, 0, len(results))
	for _, result := range results {
		chat = append(chat, chatMessage{
			Role:       "tool",
			Content:    string(result.Content),
			ToolName:   result.Name,
			ToolCallID: result.ID,
		})
	}
	return chat, nil
}

// chatResult returns why the model stopped, from the reason in the response. A
// reply which was truncated is so even when it called tools, as its calls may
// be incomplete.
func chatResult(response chatResponse) schema.ResultType {
	switch {
	case response.DoneReason == "length":
		return schema.ResultMaxTokens
	case len(response.Message.ToolCalls) > 0:
		return schema.ResultToolCall
	case response.DoneReason == "stop":
		return schema.ResultStop
	default:
		return schema.ResultOther
	}
}

// isAudio returns true if the MIME type is audio
func isAudio(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "audio/")
}

// isImage returns true if the MIME type is an image
func isImage(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "image/")
}

////////////////////////////////////////////////////////////////////////////////
// ACCEPTER

var _ llm.Accepter = (*Client)(nil)

// Accepts returns whether the model accepts an attachment of the type, which
// is images
func (*Client) Accepts(_ llm.Model, contentType string) bool {
	return isImage(contentType)
}
