package ollama

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// SESSION → OLLAMA MESSAGES

// ollamaMessagesFromSession converts a schema.Conversation to []chatMessage.
// Tool result blocks within a single message are unpacked into separate
// "tool" role messages, one per result, to match Ollama's expectations.
func ollamaMessagesFromSession(session *schema.Conversation) ([]chatMessage, error) {
	if session == nil {
		return nil, nil
	}
	msgs := make([]chatMessage, 0, len(*session))
	for _, msg := range *session {
		mm, err := ollamaChatMessagesFromMessage(msg)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, mm...)
	}
	return msgs, nil
}

// ollamaChatMessagesFromMessage converts a single schema.Message into one or
// more chatMessages. Tool results are split out as separate "tool" role messages.
func ollamaChatMessagesFromMessage(msg *schema.Message) ([]chatMessage, error) {
	if hasToolResult(msg) {
		var msgs []chatMessage
		for i := range msg.Content {
			if msg.Content[i].ToolResult == nil {
				continue
			}
			mm, err := ollamaToolResultMessage(msg.Content[i].ToolResult)
			if err != nil {
				return nil, err
			}
			msgs = append(msgs, mm)
		}
		return msgs, nil
	}

	cm, err := ollamaChatMessageFromMessage(msg)
	if err != nil {
		return nil, err
	}
	return []chatMessage{cm}, nil
}

// hasToolResult reports whether the message contains any ToolResult blocks.
func hasToolResult(msg *schema.Message) bool {
	for _, b := range msg.Content {
		if b.ToolResult != nil {
			return true
		}
	}
	return false
}

// ollamaChatMessageFromMessage converts a single schema.Message to a chatMessage.
// Text blocks are concatenated into Content; image attachments go into Images;
// tool calls go into ToolCalls.
func ollamaChatMessageFromMessage(msg *schema.Message) (chatMessage, error) {
	cm := chatMessage{Role: msg.Role}
	var textParts []string

	for i := range msg.Content {
		block := &msg.Content[i]

		// Text block
		if block.Text != nil {
			textParts = append(textParts, *block.Text)
			continue
		}

		// Thinking block
		if block.Thinking != nil {
			cm.Thinking = *block.Thinking
			continue
		}

		// Attachment — Ollama only supports image bytes via the images field.
		// Text attachments are folded into the content string.
		if block.Attachment != nil {
			if block.Attachment.IsText() && len(block.Attachment.Data) > 0 {
				textParts = append(textParts, block.Attachment.TextContent())
				continue
			}
			mediaType, _, _ := mime.ParseMediaType(block.Attachment.ContentType)
			if strings.HasPrefix(mediaType, "image/") && len(block.Attachment.Data) > 0 {
				cm.Images = append(cm.Images, block.Attachment.Data)
				continue
			}
			return chatMessage{}, fmt.Errorf("unsupported attachment type %q: Ollama only supports image/* attachments", block.Attachment.ContentType)
		}

		// Tool call (assistant message carrying a tool invocation in history)
		if block.ToolCall != nil {
			input := block.ToolCall.Input
			if len(input) == 0 {
				input = json.RawMessage("{}")
			}
			cm.ToolCalls = append(cm.ToolCalls, chatToolCall{
				ID: block.ToolCall.ID,
				Function: chatToolCallFunction{
					Name:      block.ToolCall.Name,
					Arguments: input,
				},
			})
			continue
		}
	}

	cm.Content = strings.Join(textParts, "\n")
	return cm, nil
}

// ollamaToolResultMessage creates a "tool" role chatMessage from a schema.ToolResult.
func ollamaToolResultMessage(tr *schema.ToolResult) (chatMessage, error) {
	return chatMessage{
		Role:       schema.RoleTool,
		Content:    toolResultContent(tr.Content),
		ToolCallID: tr.ID,
		ToolName:   tr.Name,
	}, nil
}

// toolResultContent converts json.RawMessage tool result content to a plain string.
// A JSON-encoded string is unwrapped; any other JSON value is used verbatim.
func toolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

///////////////////////////////////////////////////////////////////////////////
// OLLAMA RESPONSE → SCHEMA MESSAGE

// messageFromOllamaResponse converts a chatResponse to a schema.Message.
func messageFromOllamaResponse(resp *chatResponse) (*schema.Message, error) {
	blocks, err := contentBlocksFromOllamaMessage(&resp.Message)
	if err != nil {
		return nil, err
	}

	result := resultFromDoneReason(resp.DoneReason)

	// Upgrade to ResultToolCall when tool calls are present
	for _, block := range blocks {
		if block.ToolCall != nil {
			result = schema.ResultToolCall
			break
		}
	}

	return &schema.Message{
		Role:    schema.RoleAssistant,
		Content: blocks,
		Result:  result,
	}, nil
}

// contentBlocksFromOllamaMessage extracts schema.ContentBlocks from a chatMessage.
func contentBlocksFromOllamaMessage(msg *chatMessage) ([]schema.ContentBlock, error) {
	var blocks []schema.ContentBlock

	// Thinking block comes first when present
	if msg.Thinking != "" {
		thinking := msg.Thinking
		blocks = append(blocks, schema.ContentBlock{Thinking: &thinking})
	}

	// Text content
	if msg.Content != "" {
		text := msg.Content
		blocks = append(blocks, schema.ContentBlock{Text: &text})
	}

	// Tool calls
	for _, tc := range msg.ToolCalls {
		input := tc.Function.Arguments
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		blocks = append(blocks, schema.ContentBlock{
			ToolCall: &schema.ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			},
		})
	}

	// Images returned by image-generation models
	for _, img := range msg.Images {
		blocks = append(blocks, schema.ContentBlock{
			Attachment: &schema.Attachment{
				ContentType: http.DetectContentType(img),
				Data:        img,
			},
		})
	}

	return blocks, nil
}

///////////////////////////////////////////////////////////////////////////////
// TOOLS CONVERSION

// ollamaToolsFromTools converts a slice of llm.Tool to Ollama chatTools.
func ollamaToolsFromTools(tools []llm.Tool) (chatTools, error) {
	var result chatTools
	for _, t := range tools {
		s, err := t.InputSchema()
		if err != nil {
			continue
		}
		params, err := json.Marshal(s)
		if err != nil {
			continue
		}
		result = append(result, chatTool{
			Type: "function",
			Function: chatToolFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  params,
			},
		})
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// DONE REASON → RESULT TYPE

// resultFromDoneReason maps Ollama done_reason strings to schema.ResultType.
func resultFromDoneReason(reason string) schema.ResultType {
	switch reason {
	case "stop":
		return schema.ResultStop
	case "length":
		return schema.ResultMaxTokens
	case "tool_calls":
		return schema.ResultToolCall
	default:
		return schema.ResultOther
	}
}
