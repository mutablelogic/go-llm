package anthropic

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
// GLOBALS

// emptyReply is the text of a reply which had neither text nor tool calls,
// as it's sent back, as an assistant message can't be empty
const emptyReply = "(No reply.)"

// imageTypes are the types of images the API understands
var imageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newReply returns the reply as a message, with its thinking, with its
// signature, text, and tool calls as content blocks, in order, why the model
// stopped, and its output tokens
func newReply(response messagesResponse) *schema.Message {
	reply := &schema.Message{
		MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{}},
		Tokens:      uint64(response.Usage.OutputTokens),
		Result:      result(response.StopReason),
	}
	for _, block := range response.Content {
		switch block.Type {
		case "thinking":
			content := schema.ContentBlock{Thinking: types.Ptr(types.Value(block.Thinking))}
			if block.Signature != "" {
				content.Signature = types.Ptr(block.Signature)
			}
			reply.Content = append(reply.Content, content)
		case "text":
			if block.Text != "" {
				reply.Content = append(reply.Content, schema.ContentBlock{Text: types.Ptr(block.Text)})
			}
		case "tool_use":
			input := block.Input
			if len(input) == 0 {
				input = json.RawMessage(`{}`)
			}
			reply.Content = append(reply.Content, schema.ContentBlock{ToolCall: &schema.ToolCall{ID: block.ID, Name: block.Name, Input: input}})
		}
	}
	return reply
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// result returns why the model stopped, from the API's reason
func result(reason string) schema.ResultType {
	switch reason {
	case "end_turn", "stop_sequence":
		return schema.ResultStop
	case "max_tokens":
		return schema.ResultMaxTokens
	case "tool_use":
		return schema.ResultToolCall
	case "refusal":
		return schema.ResultBlocked
	case "model_context_window_exceeded":
		return schema.ResultMaxContext
	default:
		return schema.ResultOther
	}
}

// newMessages returns the messages, as the API's, and the system prompt,
// which is the system messages, as the API has a system prompt rather than
// system messages. Thinking messages are left out. A message whose role is
// the same as the one before it is added to it, as the API's roles take
// turns, and the results of tools are the user's. Thinking is only sent back
// in the replies after the last user message, which are the turn the model
// is calling tools in, as the API needs it there, and drops it otherwise:
// earlier thinking, from the start of the conversation, can be left out.
func newMessages(ctx context.Context, messages []llm.Message) ([]message, string, error) {
	// The replies after the last message from the user, rather than with the
	// results of tools, are this turn's
	turn := 0
	for i, m := range messages {
		if m != nil && m.Role() == "user" && len(m.ToolResults()) == 0 {
			turn = i
		}
	}

	var system []string
	result := make([]message, 0, len(messages))
	for i, m := range messages {
		if m == nil {
			return nil, "", llm.ErrBadParameter.With("missing message")
		}
		var role string
		var blocks []contentBlock
		switch m.Role() {
		case "system":
			if text := strings.TrimSpace(m.Text()); text != "" {
				system = append(system, text)
			}
			continue
		case "thinking":
			continue
		case "user", "tool":
			role = "user"
			var err error
			if blocks, err = userBlocks(ctx, m); err != nil {
				return nil, "", err
			}
		case "assistant":
			role = "assistant"
			blocks = assistantBlocks(m, i > turn)
		default:
			return nil, "", llm.ErrBadParameter.Withf("unsupported message role %q", m.Role())
		}
		if len(blocks) == 0 {
			continue
		}
		if n := len(result); n > 0 && result[n-1].Role == role {
			result[n-1].Content = append(result[n-1].Content, blocks...)
		} else {
			result = append(result, message{Role: role, Content: blocks})
		}
	}
	if len(result) == 0 {
		return nil, "", llm.ErrBadParameter.With("missing messages")
	}
	return result, strings.Join(system, "\n\n"), nil
}

// userBlocks returns the content of a message from the user, or of the
// results of tools: the results first, as the API needs them before the rest
// of the message, then its images and documents, which are read, and its
// text
func userBlocks(ctx context.Context, m llm.Message) ([]contentBlock, error) {
	var blocks []contentBlock
	for _, r := range m.ToolResults() {
		content := string(r.Content)
		var text string
		if json.Unmarshal(r.Content, &text) == nil {
			content = text
		}
		if content == "" {
			content = "(No result.)"
		}
		blocks = append(blocks, contentBlock{Type: "tool_result", ToolUseID: r.ID, Content: content, IsError: r.IsError})
	}
	for _, resource := range m.Resources() {
		block, err := resourceBlock(ctx, resource)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	if text := m.Text(); strings.TrimSpace(text) != "" {
		blocks = append(blocks, contentBlock{Type: "text", Text: text})
	}
	return blocks, nil
}

// assistantBlocks returns the content of a reply: its thinking, when it's
// signed, and sent back, then its text and the tools it called, or text
// which says there's no reply, when there's neither, as the API doesn't
// accept an empty reply
func assistantBlocks(m llm.Message, withThinking bool) []contentBlock {
	var blocks []contentBlock
	if reply, ok := m.(*schema.Message); ok && withThinking {
		for _, block := range reply.Content {
			if block.Thinking != nil && block.Signature != nil && *block.Signature != "" {
				blocks = append(blocks, contentBlock{Type: "thinking", Thinking: types.Ptr(*block.Thinking), Signature: *block.Signature})
			}
		}
	}
	thinking := len(blocks)
	if text := m.Text(); strings.TrimSpace(text) != "" {
		blocks = append(blocks, contentBlock{Type: "text", Text: text})
	}
	for _, call := range m.ToolCalls() {
		input := call.Input
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		blocks = append(blocks, contentBlock{Type: "tool_use", ID: call.ID, Name: call.Name, Input: input})
	}
	if len(blocks) == thinking {
		blocks = append(blocks, contentBlock{Type: "text", Text: emptyReply})
	}
	return blocks
}

// resourceBlock returns the resource as an image, or a document, which is a
// PDF, or text, as the API understands no other resources
func resourceBlock(ctx context.Context, resource llm.Resource) (contentBlock, error) {
	mediaType, _, err := mime.ParseMediaType(resource.Type())
	if err != nil {
		return contentBlock{}, llm.ErrBadParameter.Withf("invalid resource type %q", resource.Type())
	}
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return contentBlock{}, err
	}
	switch {
	case imageTypes[mediaType]:
		return contentBlock{Type: "image", Source: &source{Type: "base64", MediaType: mediaType, Data: base64.StdEncoding.EncodeToString(data)}}, nil
	case mediaType == "application/pdf":
		return contentBlock{Type: "document", Source: &source{Type: "base64", MediaType: mediaType, Data: base64.StdEncoding.EncodeToString(data)}}, nil
	case strings.HasPrefix(mediaType, "text/"):
		return contentBlock{Type: "document", Source: &source{Type: "text", MediaType: "text/plain", Data: string(data)}}, nil
	default:
		return contentBlock{}, llm.ErrBadParameter.Withf("unsupported resource type %q", mediaType)
	}
}
