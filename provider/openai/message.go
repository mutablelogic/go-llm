package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"mime"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newChatResponse returns the message in a choice of a response from
// POST /chat/completions, with its reasoning, text and tool calls as content
// blocks, and why the model stopped. Its reasoning is the reasoning content,
// or Mistral's thinking parts of its content. A refusal is its text, and the
// reply is blocked. Its tokens are those returned, without reasoning which isn't.
func newChatResponse(choice chatChoice, usage *chatUsage) *schema.Message {
	chat := choice.Message
	message := &schema.Message{
		Sender:  "assistant",
		Content: []schema.ContentBlock{},
		Result:  chatResult(choice),
	}
	if usage := replyUsage(choice, usage); usage.OutputTokens > 0 {
		message.Tokens = uint64(usage.OutputTokens - min(usage.ReasoningTokens, usage.OutputTokens))
	}
	if thinking := chat.ReasoningContent + contentThinking(chat.Content); thinking != "" {
		message.Content = append(message.Content, schema.ContentBlock{Thinking: types.Ptr(thinking)})
	}
	if text := contentText(chat.Content); text != "" {
		message.Content = append(message.Content, schema.ContentBlock{Text: types.Ptr(text)})
	}
	if chat.Refusal != "" {
		message.Content = append(message.Content, schema.ContentBlock{Text: types.Ptr(chat.Refusal)})
	}
	for _, call := range chat.ToolCalls {
		message.Content = append(message.Content, schema.ContentBlock{ToolCall: &schema.ToolCall{
			ID:    call.ID,
			Name:  call.Function.Name,
			Input: toolInput(call.Function.Arguments),
		}})
	}
	return message
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// emptyReply is the text of a reply which had neither text nor tool calls,
// as it's sent back
const emptyReply = "(No reply.)"

// thinker is a message with thinking, which is sent back to servers which
// need it
type thinker interface {
	Thinking() string
}

// chatMessages converts messages to the wire format for
// POST /chat/completions. A message with more than one tool result is more
// than one tool message. Thinking is only sent back when reasoning is true,
// for servers which need it, such as DeepSeek with tools.
func chatMessages(ctx context.Context, messages []llm.Message, reasoning bool) ([]chatMessage, error) {
	result := make([]chatMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			return nil, llm.ErrBadParameter.With("missing message")
		}
		chat, err := newChatMessages(ctx, message, reasoning)
		if err != nil {
			return nil, err
		}
		result = append(result, chat...)
	}
	return result, nil
}

// newChatMessages converts a message to the wire format for
// POST /chat/completions. Images are read and sent inline, as data URLs, with
// the text as parts, as only images are accepted. An assistant message's tool
// calls are sent with it, and each tool result is a tool message. Thinking
// is only sent with an assistant message when reasoning is true, as DeepSeek
// needs every reply's reasoning when there are tools, and other servers
// don't accept it.
func newChatMessages(ctx context.Context, message llm.Message, reasoning bool) ([]chatMessage, error) {
	var chat chatMessage
	switch role := message.Role(); role {
	case "user", "assistant", "system":
		chat.Role, chat.Content = role, message.Text()
	case "thinking":
		return nil, nil
	case "tool":
		return toolMessages(message), nil
	default:
		return nil, llm.ErrBadParameter.Withf("unsupported message role %q", role)
	}
	if resources := message.Resources(); len(resources) > 0 {
		parts := []chatPart{}
		if text := message.Text(); text != "" {
			parts = append(parts, chatPart{Type: "text", Text: text})
		}
		for _, resource := range resources {
			if !isImage(resource.Type()) {
				return nil, llm.ErrBadParameter.Withf("unsupported resource type %q", resource.Type())
			}
			data, err := llm.ReadAll(ctx, resource)
			if err != nil {
				return nil, err
			}
			parts = append(parts, chatPart{Type: "image_url", ImageURL: &chatImageURL{
				URL: "data:" + resource.Type() + ";base64," + base64.StdEncoding.EncodeToString(data),
			}})
		}
		chat.Content = parts
	}
	for _, call := range message.ToolCalls() {
		if chat.Role != "assistant" {
			return nil, llm.ErrBadParameter.Withf("tool calls can't be in a %q message", chat.Role)
		}
		arguments := string(call.Input)
		if arguments == "" {
			arguments = "{}"
		}
		chat.ToolCalls = append(chat.ToolCalls, chatToolCall{
			ID:       call.ID,
			Type:     "function",
			Function: chatToolCallFunction{Name: call.Name, Arguments: arguments},
		})
	}

	// A reply with only tool calls has no content, and a reply with neither,
	// such as one which ran out of tokens as it was thinking, has text which
	// says so, as servers, such as Mistral, reject an assistant message
	// without either, and it can't be left out, as Mistral rejects a user's
	// message after a tool's result, too. A reply has its thinking for
	// servers which need it.
	if len(chat.ToolCalls) > 0 && chat.Content == "" {
		chat.Content = nil
	} else if chat.Role == "assistant" && chat.Content == "" {
		chat.Content = emptyReply
	}
	if thinking, ok := message.(thinker); ok && reasoning && chat.Role == "assistant" {
		chat.ReasoningContent = thinking.Thinking()
	}
	return []chatMessage{chat}, nil
}

// toolMessages returns a tool message for each tool result in the message,
// with the result as its content, or the message's text when it has no
// results. An error is sent as its result.
func toolMessages(message llm.Message) []chatMessage {
	results := message.ToolResults()
	if len(results) == 0 {
		return []chatMessage{{Role: "tool", Content: message.Text()}}
	}
	chat := make([]chatMessage, 0, len(results))
	for _, result := range results {
		chat = append(chat, chatMessage{
			Role:       "tool",
			Content:    string(result.Content),
			ToolCallID: result.ID,
		})
	}
	return chat
}

// replyUsage returns the tokens used by the reply in the choice. Its
// reasoning tokens are those of reasoning which isn't returned, as with
// OpenAI's reasoning models, so they're none when the reply has its
// reasoning, as DeepSeek's do, which is sent back with tools, and Mistral's,
// as thinking parts of its content.
func replyUsage(choice chatChoice, usage *chatUsage) llm.Usage {
	result := usage.Usage()
	if choice.Message.ReasoningContent != "" || contentThinking(choice.Message.Content) != "" {
		result.ReasoningTokens = 0
	}
	return result
}

// chatResult returns why the model stopped, from the reason in the choice. A
// reply which didn't finish, as it was truncated or filtered, is so even when
// it called tools, as its calls may be incomplete.
func chatResult(choice chatChoice) schema.ResultType {
	switch {
	case choice.Message.Refusal != "", choice.FinishReason == "content_filter":
		return schema.ResultBlocked
	case choice.FinishReason == "length":
		return schema.ResultMaxTokens
	case len(choice.Message.ToolCalls) > 0:
		return schema.ResultToolCall
	case choice.FinishReason == "stop":
		return schema.ResultStop
	default:
		return schema.ResultOther
	}
}

// contentThinking returns the thinking of a message's content, which is
// parts, of which the thinking parts' text parts are joined, as Mistral
// returns a reasoning model's thinking, or empty when there's none
// Reference: https://docs.mistral.ai/capabilities/reasoning
func contentThinking(content any) string {
	parts, ok := content.([]any)
	if !ok {
		return ""
	}
	var thinking strings.Builder
	for _, part := range parts {
		if part, ok := part.(map[string]any); ok && part["type"] == "thinking" {
			thinking.WriteString(contentText(part["thinking"]))
		}
	}
	return thinking.String()
}

// contentText returns the text of a message's content, which is a string,
// or parts, of which the text parts are joined
func contentText(content any) string {
	switch content := content.(type) {
	case string:
		return content
	case []any:
		var text strings.Builder
		for _, part := range content {
			if part, ok := part.(map[string]any); ok && part["type"] == "text" {
				if value, ok := part["text"].(string); ok {
					text.WriteString(value)
				}
			}
		}
		return text.String()
	default:
		return ""
	}
}

// toolInput returns a tool call's arguments as JSON, which are empty when
// there are none. Arguments which aren't JSON, which a model can generate,
// are a JSON string, so the tool fails with an error the model can see.
func toolInput(arguments string) json.RawMessage {
	if strings.TrimSpace(arguments) == "" {
		return nil
	} else if json.Valid([]byte(arguments)) {
		return json.RawMessage(arguments)
	}
	data, _ := json.Marshal(arguments)
	return data
}

// isImage returns true if the MIME type is an image
func isImage(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "image/")
}
