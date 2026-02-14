package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// SESSION → ANTHROPIC MESSAGES

// anthropicMessagesFromSession converts a schema.Conversation to Anthropic message format.
// System messages are skipped (handled separately via the system parameter).
func anthropicMessagesFromSession(session *schema.Conversation) ([]anthropicMessage, error) {
	if session == nil {
		return nil, nil
	}

	messages := make([]anthropicMessage, 0, len(*session))
	for _, msg := range *session {
		if msg.Role == schema.RoleSystem {
			continue
		}
		am, err := anthropicMessageFromMessage(msg)
		if err != nil {
			return nil, err
		}
		messages = append(messages, am)
	}
	return messages, nil
}

// anthropicMessageFromMessage converts a single schema.Message to Anthropic format.
// If the message has meta["thought"]=true, the first text block becomes a thinking block.
func anthropicMessageFromMessage(msg *schema.Message) (anthropicMessage, error) {
	blocks := make([]anthropicContentBlock, 0, len(msg.Content))

	// Check if this message contains thinking content
	hasThought := false
	if msg.Meta != nil {
		if thought, ok := msg.Meta["thought"].(bool); ok && thought {
			hasThought = true
		}
	}

	firstText := true
	for i := range msg.Content {
		block := &msg.Content[i]

		// First text block in a thinking message → thinking block
		if block.Text != nil && hasThought && firstText {
			firstText = false
			ab := anthropicContentBlock{
				Type:     blockTypeThinking,
				Thinking: *block.Text,
			}
			if sig, ok := msg.Meta["thought_signature"].(string); ok {
				ab.Signature = sig
			}
			blocks = append(blocks, ab)
			continue
		}

		ab, err := anthropicBlockFromContentBlock(block)
		if err != nil {
			return anthropicMessage{}, err
		}
		if ab != nil {
			blocks = append(blocks, *ab)
		}
	}

	return anthropicMessage{
		Role:    msg.Role,
		Content: blocks,
	}, nil
}

// anthropicBlockFromContentBlock converts a schema.ContentBlock to an Anthropic content block
func anthropicBlockFromContentBlock(block *schema.ContentBlock) (*anthropicContentBlock, error) {
	// Text content
	if block.Text != nil {
		return &anthropicContentBlock{
			Type: blockTypeText,
			Text: *block.Text,
		}, nil
	}

	// Attachment (image, document)
	if block.Attachment != nil {
		return anthropicBlockFromAttachment(block.Attachment)
	}

	// Tool call (model requesting tool use)
	if block.ToolCall != nil {
		return &anthropicContentBlock{
			Type:  blockTypeToolUse,
			ID:    block.ToolCall.ID,
			Name:  block.ToolCall.Name,
			Input: block.ToolCall.Input,
		}, nil
	}

	// Tool result (user providing tool response)
	if block.ToolResult != nil {
		ab := &anthropicContentBlock{
			Type:      blockTypeToolResult,
			ToolUseID: block.ToolResult.ID,
			IsError:   block.ToolResult.IsError,
		}
		if len(block.ToolResult.Content) > 0 {
			// If content is a JSON string, pass through directly.
			// Otherwise, quote the raw JSON as a string for Anthropic.
			if block.ToolResult.Content[0] == '"' {
				ab.Content = block.ToolResult.Content
			} else {
				text := string(block.ToolResult.Content)
				data, _ := json.Marshal(text)
				ab.Content = data
			}
		}
		return ab, nil
	}

	return nil, nil
}

// anthropicBlockFromAttachment converts an Attachment to an Anthropic content block
func anthropicBlockFromAttachment(att *schema.Attachment) (*anthropicContentBlock, error) {
	if len(att.Data) > 0 {
		// Base64-encoded inline data
		blockType := blockTypeImage
		if strings.HasPrefix(att.Type, "application/pdf") {
			blockType = blockTypeDocument
		}
		return &anthropicContentBlock{
			Type: blockType,
			Source: &anthropicSource{
				Type:      sourceTypeBase64,
				MediaType: att.Type,
				Data:      base64.StdEncoding.EncodeToString(att.Data),
			},
		}, nil
	}
	if att.URL != nil {
		return &anthropicContentBlock{
			Type: blockTypeImage,
			Source: &anthropicSource{
				Type: sourceTypeURL,
				URL:  att.URL.String(),
			},
		}, nil
	}
	return nil, nil
}

///////////////////////////////////////////////////////////////////////////////
// ANTHROPIC RESPONSE → SCHEMA MESSAGE

// messageFromAnthropicResponse converts an Anthropic API response to a schema.Message
func messageFromAnthropicResponse(role string, content []anthropicContentBlock, stopReason string) (*schema.Message, error) {
	var blocks []schema.ContentBlock
	var meta map[string]any

	for _, ab := range content {
		block, blockMeta := contentBlockFromAnthropicBlock(&ab)
		blocks = append(blocks, block)
		if blockMeta != nil {
			if meta == nil {
				meta = make(map[string]any)
			}
			for k, v := range blockMeta {
				meta[k] = v
			}
		}
	}

	return &schema.Message{
		Role:    role,
		Content: blocks,
		Result:  resultFromStopReason(stopReason),
		Meta:    meta,
	}, nil
}

// contentBlockFromAnthropicBlock converts an Anthropic content block to a schema.ContentBlock
func contentBlockFromAnthropicBlock(ab *anthropicContentBlock) (schema.ContentBlock, map[string]any) {
	switch ab.Type {
	case blockTypeText:
		return schema.ContentBlock{
			Text: &ab.Text,
		}, nil

	case blockTypeThinking:
		meta := map[string]any{"thought": true}
		if ab.Signature != "" {
			meta["thought_signature"] = ab.Signature
		}
		return schema.ContentBlock{
			Text: &ab.Thinking,
		}, meta

	case blockTypeToolUse:
		return schema.ContentBlock{
			ToolCall: &schema.ToolCall{
				ID:    ab.ID,
				Name:  ab.Name,
				Input: ab.Input,
			},
		}, nil

	case blockTypeToolResult:
		return schema.ContentBlock{
			ToolResult: &schema.ToolResult{
				ID:      ab.ToolUseID,
				Content: ab.Content,
				IsError: ab.IsError,
			},
		}, nil

	case blockTypeImage, blockTypeDocument:
		if ab.Source != nil {
			att := attachmentFromSource(ab.Source)
			if att != nil {
				return schema.ContentBlock{Attachment: att}, nil
			}
		}
	}

	// Unknown type - return empty text block
	empty := ""
	return schema.ContentBlock{Text: &empty}, nil
}

// attachmentFromSource converts an Anthropic source to a schema.Attachment
func attachmentFromSource(src *anthropicSource) *schema.Attachment {
	if src.Type == sourceTypeBase64 && src.Data != "" {
		data, err := base64.StdEncoding.DecodeString(src.Data)
		if err != nil {
			return nil
		}
		return &schema.Attachment{
			Type: src.MediaType,
			Data: data,
		}
	}
	if src.Type == sourceTypeURL && src.URL != "" {
		u, err := url.Parse(src.URL)
		if err != nil {
			return nil
		}
		return &schema.Attachment{
			Type: src.MediaType,
			URL:  u,
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// TOOLS CONVERSION

// anthropicToolsFromToolkit converts a tool.Toolkit to Anthropic tool JSON payloads
func anthropicToolsFromToolkit(tk *tool.Toolkit) ([]json.RawMessage, error) {
	var result []json.RawMessage
	for _, t := range tk.Tools() {
		s, err := t.Schema()
		if err != nil {
			continue
		}
		data, err := json.Marshal(struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			InputSchema any    `json:"input_schema"`
		}{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: s,
		})
		if err != nil {
			continue
		}
		result = append(result, json.RawMessage(data))
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// STOP REASON → RESULT TYPE

// resultFromStopReason maps Anthropic stop reasons to schema.ResultType
func resultFromStopReason(reason string) schema.ResultType {
	switch reason {
	case stopReasonEndTurn, stopReasonStopSequence:
		return schema.ResultStop
	case stopReasonMaxTokens:
		return schema.ResultMaxTokens
	case stopReasonToolUse:
		return schema.ResultToolCall
	case stopReasonRefusal:
		return schema.ResultBlocked
	default:
		return schema.ResultOther
	}
}
