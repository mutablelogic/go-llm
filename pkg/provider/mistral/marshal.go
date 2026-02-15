package mistral

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"mime"
	"strings"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// SESSION → MISTRAL MESSAGES

// mistralMessagesFromSession converts a schema.Conversation to Mistral message format.
// System messages are kept (Mistral handles them natively in the messages array).
// Tool result messages are split so each carries exactly one tool_call_id.
func mistralMessagesFromSession(session *schema.Conversation) ([]mistralMessage, error) {
	if session == nil {
		return nil, nil
	}

	// pendingIDs is a FIFO queue of generated IDs for tool calls whose
	// original IDs were invalid. Tool-result messages consume from this
	// queue so that each result references the same replacement ID as
	// the tool call it corresponds to.
	var pendingIDs []string

	messages := make([]mistralMessage, 0, len(*session))
	for _, msg := range *session {
		// Tool-result messages must be split: Mistral requires one message per
		// tool_call_id, so a schema.Message with multiple ToolResult blocks
		// becomes multiple mistralMessages with role "tool".
		if hasToolResult(msg) {
			for i := range msg.Content {
				if msg.Content[i].ToolResult == nil {
					continue
				}
				mm, err := mistralToolResultMessage(msg.Content[i].ToolResult)
				if err != nil {
					return nil, err
				}
				// If the tool_call_id is invalid, consume the next pending
				// ID that was generated for the matching tool call.
				if !isValidMistralID(mm.ToolCallID) {
					if len(pendingIDs) > 0 {
						mm.ToolCallID = pendingIDs[0]
						pendingIDs = pendingIDs[1:]
					} else {
						mm.ToolCallID = generateMistralID()
					}
				}
				messages = append(messages, mm)
			}
			continue
		}

		mms, err := mistralMessagesFromMessage(msg)
		if err != nil {
			return nil, err
		}

		// Skip empty assistant messages (no content, no tool calls) — these
		// can occur when another provider (e.g. Gemini) returns a tool call
		// response with no accompanying text.
		filtered := mms[:0]
		for _, mm := range mms {
			if mm.Role == roleAssistant && len(mm.ToolCalls) == 0 {
				if s, ok := mm.Content.(string); ok && s == "" {
					continue
				}
			}
			filtered = append(filtered, mm)
		}
		if len(filtered) == 0 {
			continue
		}

		// Replace any invalid tool call IDs and queue the generated IDs
		// so that the subsequent tool-result messages can reference them.
		for i := range filtered {
			for j := range filtered[i].ToolCalls {
				if !isValidMistralID(filtered[i].ToolCalls[j].Id) {
					newID := generateMistralID()
					filtered[i].ToolCalls[j].Id = newID
					pendingIDs = append(pendingIDs, newID)
				}
			}
		}

		messages = append(messages, filtered...)
	}
	return messages, nil
}

///////////////////////////////////////////////////////////////////////////////
// SCHEMA MESSAGE → MISTRAL MESSAGE (OUTBOUND)

// mistralMessagesFromMessage converts a single schema.Message to one or more
// Mistral messages. Mistral requires that an assistant message has either
// content OR tool_calls, never both. When a schema.Message contains both
// text and tool calls (common with streaming), the function splits it into
// two messages: first the text content, then the tool calls with empty content.
func mistralMessagesFromMessage(msg *schema.Message) ([]mistralMessage, error) {
	// Collect tool calls from content blocks
	var toolCalls []mistralToolCall
	var parts []contentPart
	var singleText *string

	textCount := 0
	otherCount := 0

	for i := range msg.Content {
		block := &msg.Content[i]

		if block.Text != nil {
			textCount++
			singleText = block.Text
			parts = append(parts, contentPart{
				Type: "text",
				Text: *block.Text,
			})
			continue
		}

		if block.Attachment != nil {
			otherCount++
			mediaType, _, _ := mime.ParseMediaType(block.Attachment.Type)
			isAudio := strings.HasPrefix(mediaType, "audio/")
			// Text attachments → text content part
			if block.Attachment.IsText() && len(block.Attachment.Data) > 0 {
				text := block.Attachment.TextContent()
				textCount++
				singleText = &text
				parts = append(parts, contentPart{
					Type: "text",
					Text: text,
				})
			} else if isAudio && len(block.Attachment.Data) > 0 {
				// Audio data → input_audio (base64)
				parts = append(parts, contentPart{
					Type:       "input_audio",
					InputAudio: base64.StdEncoding.EncodeToString(block.Attachment.Data),
				})
			} else if isAudio && block.Attachment.URL != nil && block.Attachment.URL.Scheme != "file" {
				// Remote audio URL → input_audio (URL string)
				parts = append(parts, contentPart{
					Type:       "input_audio",
					InputAudio: block.Attachment.URL.String(),
				})
			} else if len(block.Attachment.Data) > 0 {
				// Image (or other binary) data → data: URI
				if !strings.HasPrefix(mediaType, "image/") {
					return nil, fmt.Errorf("unsupported attachment type %q: only image/*, audio/*, and text/* are supported", block.Attachment.Type)
				}
				dataURI := "data:" + block.Attachment.Type + ";base64," + base64.StdEncoding.EncodeToString(block.Attachment.Data)
				parts = append(parts, contentPart{
					Type:     "image_url",
					ImageURL: &imageURL{URL: dataURI},
				})
			} else if block.Attachment.URL != nil && block.Attachment.URL.Scheme != "file" {
				// Remote URL (https, etc.) — for images
				parts = append(parts, contentPart{
					Type:     "image_url",
					ImageURL: &imageURL{URL: block.Attachment.URL.String()},
				})
			} else {
				return nil, fmt.Errorf("unsupported attachment: no data and no remote URL")
			}
			continue
		}

		if block.ToolCall != nil {
			tc := mistralToolCall{
				Id:   block.ToolCall.ID,
				Type: "function",
				Function: mistralFunction{
					Name: block.ToolCall.Name,
				},
			}
			if len(block.ToolCall.Input) > 0 {
				tc.Function.Arguments = string(block.ToolCall.Input)
			}
			toolCalls = append(toolCalls, tc)
			continue
		}
	}

	// Build content value.
	// Mistral only supports the multi-part content array ([{"type":"text",...}])
	// for user and system messages. Assistant messages MUST use a plain string.
	// When an assistant message has multiple text blocks (common with streaming),
	// we concatenate them into a single string.
	var content any
	if msg.Role == roleAssistant {
		// Assistant: always a plain string (concatenate all text parts)
		var sb strings.Builder
		for _, p := range parts {
			if p.Type == "text" {
				sb.WriteString(p.Text)
			}
		}
		content = sb.String()
	} else if textCount == 1 && otherCount == 0 {
		// Non-assistant with single text block → plain string
		content = *singleText
	} else if len(parts) > 0 {
		// Non-assistant with multiple blocks → []contentPart array
		content = parts
	} else {
		content = ""
	}

	hasToolCalls := len(toolCalls) > 0

	// Mistral assistant messages must have EITHER content OR tool_calls, never
	// both, AND Mistral does not allow consecutive assistant messages. When a
	// schema message contains both text and tool calls (common with streaming),
	// we keep only the tool calls. The text is pre-call reasoning that is not
	// essential for conversation replay.
	if msg.Role == roleAssistant {
		if hasToolCalls {
			// Tool calls take priority — Content stays nil (omitted via omitempty)
			return []mistralMessage{{Role: msg.Role, ToolCalls: toolCalls}}, nil
		}
		// Content only
		return []mistralMessage{{Role: msg.Role, Content: content}}, nil
	}

	mm := mistralMessage{
		Role:    msg.Role,
		Content: content,
	}
	if hasToolCalls {
		mm.ToolCalls = toolCalls
	}

	return []mistralMessage{mm}, nil
}

// mistralToolResultMessage creates a Mistral "tool" role message from a ToolResult.
func mistralToolResultMessage(tr *schema.ToolResult) (mistralMessage, error) {
	var content string
	if len(tr.Content) > 0 {
		content = string(tr.Content)
	}
	return mistralMessage{
		Role:       roleTool,
		Content:    content,
		ToolCallID: tr.ID,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// MISTRAL RESPONSE → SCHEMA MESSAGE (INBOUND)

// messageFromMistralResponse converts a Mistral chat completion response to a schema.Message.
func messageFromMistralResponse(resp *chatCompletionResponse) (*schema.Message, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return &schema.Message{}, nil
	}

	choice := resp.Choices[0]
	return messageFromMistralChoice(&choice)
}

// messageFromMistralChoice converts a single chat choice to a schema.Message.
func messageFromMistralChoice(choice *chatChoice) (*schema.Message, error) {
	msg := &choice.Message
	blocks, err := contentBlocksFromMistralMessage(msg)
	if err != nil {
		return nil, err
	}

	result := resultFromFinishReason(choice.FinishReason)

	// Upgrade to ResultToolCall if tool calls present
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

// contentBlocksFromMistralMessage extracts schema.ContentBlocks from a mistralMessage.
func contentBlocksFromMistralMessage(msg *mistralMessage) ([]schema.ContentBlock, error) {
	var blocks []schema.ContentBlock

	// Parse content — can be string or []contentPart
	switch v := msg.Content.(type) {
	case string:
		if v != "" {
			blocks = append(blocks, schema.ContentBlock{Text: &v})
		}
	case []any:
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			partType, _ := m["type"].(string)
			switch partType {
			case "text":
				if text, ok := m["text"].(string); ok {
					blocks = append(blocks, schema.ContentBlock{Text: &text})
				}
			case "input_audio":
				if audio, ok := m["input_audio"].(string); ok {
					data, err := base64.StdEncoding.DecodeString(audio)
					if err != nil {
						return nil, fmt.Errorf("failed to decode input_audio: %w", err)
					}
					blocks = append(blocks, schema.ContentBlock{
						Attachment: &schema.Attachment{
							Type: "audio/mpeg",
							Data: data,
						},
					})
				}
			}
		}
	}

	// Convert tool calls
	for _, tc := range msg.ToolCalls {
		input := json.RawMessage(tc.Function.Arguments)
		blocks = append(blocks, schema.ContentBlock{
			ToolCall: &schema.ToolCall{
				ID:    tc.Id,
				Name:  tc.Function.Name,
				Input: input,
			},
		})
	}

	return blocks, nil
}

///////////////////////////////////////////////////////////////////////////////
// TOOL CONVERSION

// mistralToolsFromToolkit converts a tool.Toolkit to Mistral tool definitions.
func mistralToolsFromToolkit(tk *tool.Toolkit) ([]toolDefinition, error) {
	var result []toolDefinition
	for _, t := range tk.Tools() {
		s, err := t.Schema()
		if err != nil {
			continue
		}
		data, err := json.Marshal(s)
		if err != nil {
			continue
		}
		result = append(result, toolDefinition{
			Type: "function",
			Function: toolFunctionDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  data,
			},
		})
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// FINISH REASON → RESULT TYPE

// resultFromFinishReason maps Mistral finish reasons to schema.ResultType.
func resultFromFinishReason(reason string) schema.ResultType {
	switch reason {
	case finishReasonStop:
		return schema.ResultStop
	case finishReasonLength, finishReasonModelLength:
		return schema.ResultMaxTokens
	case finishReasonToolCalls:
		return schema.ResultToolCall
	case finishReasonError:
		return schema.ResultError
	default:
		return schema.ResultOther
	}
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// mistralIDLength is the exact length Mistral requires for tool call IDs.
const mistralIDLength = 9

// mistralIDChars are the characters allowed in a Mistral tool call ID.
const mistralIDChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// isValidMistralID reports whether id satisfies Mistral's requirement:
// exactly 9 characters, each in [a-zA-Z0-9].
func isValidMistralID(id string) bool {
	if len(id) != mistralIDLength {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// generateMistralID creates a random 9-character alphanumeric ID.
func generateMistralID() string {
	b := make([]byte, mistralIDLength)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(mistralIDChars))))
		b[i] = mistralIDChars[n.Int64()]
	}
	return string(b)
}

// hasToolResult reports whether any content block is a tool result.
func hasToolResult(msg *schema.Message) bool {
	for _, b := range msg.Content {
		if b.ToolResult != nil {
			return true
		}
	}
	return false
}

// stringifyContent marshals the content field for debugging.
func stringifyContent(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
