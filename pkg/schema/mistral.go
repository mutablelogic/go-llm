package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MistralMessage wraps a universal Message for Mistral-specific JSON marshaling
type MistralMessage struct {
	Message
}

// mistralContentBlock represents Mistral's JSON format for content blocks
type mistralContentBlock struct {
	Type     string      `json:"type"`
	Text     *string     `json:"text,omitempty"`
	ImageURL interface{} `json:"image_url,omitempty"` // string or {url: string}
}

type mistralToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

type mistralWireMessage struct {
	Role       string                 `json:"role"`
	Content    interface{}            `json:"content,omitempty"` // string or []mistralContentBlock
	ToolCalls  []mistralToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Name       string                 `json:"name,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// MARSHALING

// MarshalJSON converts the universal Message to Mistral's JSON format
func (mm MistralMessage) MarshalJSON() ([]byte, error) {
	var toolCalls []mistralToolCall
	var toolCallID string
	var contentBlocks []mistralContentBlock
	var textParts []string
	hasImages := false

	for _, block := range mm.Content {
		switch block.Type {
		case ContentTypeText:
			if block.Text != nil && *block.Text != "" {
				textParts = append(textParts, *block.Text)
				text := *block.Text
				contentBlocks = append(contentBlocks, mistralContentBlock{Type: "text", Text: &text})
			}

		case ContentTypeImage:
			if block.ImageSource == nil {
				return nil, fmt.Errorf("mistral: image block missing source")
			}
			hasImages = true
			switch block.ImageSource.Type {
			case "url":
				if block.ImageSource.URL == nil || *block.ImageSource.URL == "" {
					return nil, fmt.Errorf("mistral: image url missing")
				}
				contentBlocks = append(contentBlocks, mistralContentBlock{
					Type:     "image_url",
					ImageURL: map[string]string{"url": *block.ImageSource.URL},
				})
			case "base64":
				if block.ImageSource.Data == nil || *block.ImageSource.Data == "" {
					return nil, fmt.Errorf("mistral: image base64 data missing")
				}
				mt := block.ImageSource.MediaType
				if mt == "" {
					mt = "image/jpeg"
				}
				dataURL := "data:" + mt + ";base64," + *block.ImageSource.Data
				contentBlocks = append(contentBlocks, mistralContentBlock{
					Type:     "image_url",
					ImageURL: map[string]string{"url": dataURL},
				})
			default:
				return nil, fmt.Errorf("mistral: unsupported image source type %q", block.ImageSource.Type)
			}

		case ContentTypeToolUse:
			if block.ToolName == nil {
				return nil, fmt.Errorf("mistral: tool_use requires tool name")
			}
			if block.ToolInput == nil {
				return nil, fmt.Errorf("mistral: tool_use requires tool input")
			}
			id := ""
			if block.ToolUseID != nil {
				id = *block.ToolUseID
			}
			call := mistralToolCall{ID: id, Type: "function"}
			call.Function.Name = *block.ToolName
			call.Function.Arguments = string(block.ToolInput)
			toolCalls = append(toolCalls, call)

		case ContentTypeToolResult:
			if block.ToolResultID != nil {
				toolCallID = *block.ToolResultID
			}
			if len(block.ToolResultContent) > 0 {
				textParts = append(textParts, string(block.ToolResultContent))
			} else if block.Text != nil {
				textParts = append(textParts, *block.Text)
			}

		default:
			return nil, fmt.Errorf("mistral: unsupported content type %q", block.Type)
		}
	}

	// Validate tool_calls placement
	if len(toolCalls) > 0 && mm.Role != MessageRoleAssistant {
		return nil, fmt.Errorf("mistral: tool_calls must be authored by an assistant message")
	}
	if toolCallID != "" && mm.Role != MessageRoleTool {
		return nil, fmt.Errorf("mistral: tool results must use role=tool")
	}

	// Build wire message
	wire := mistralWireMessage{
		Role:       mm.Role,
		ToolCalls:  toolCalls,
		ToolCallID: toolCallID,
	}

	// Set content: use array if images present, string otherwise
	if hasImages {
		wire.Content = contentBlocks
	} else {
		content := strings.Join(textParts, "\n\n")
		if content != "" || len(toolCalls) > 0 {
			wire.Content = content
		}
	}

	// Include tool name for role=tool messages
	if mm.Role == MessageRoleTool && len(contentBlocks) > 0 {
		// Try to infer tool name from context (optional)
		// For now, leave empty unless explicitly set
	}

	return json.Marshal(wire)
}

// UnmarshalJSON converts Mistral's JSON format to the universal Message
func (mm *MistralMessage) UnmarshalJSON(data []byte) error {
	var wire struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content,omitempty"`
		ToolCalls  []mistralToolCall `json:"tool_calls,omitempty"`
		ToolCallID string          `json:"tool_call_id,omitempty"`
		Name       string          `json:"name,omitempty"`
	}

	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("invalid mistral message: %w", err)
	}

	var contentBlocks []ContentBlock

	// Decode content (string or array of blocks)
	if len(wire.Content) > 0 && wire.Role != MessageRoleTool {
		// Try as simple string first
		var text string
		if err := json.Unmarshal(wire.Content, &text); err == nil {
			if text != "" {
				contentBlocks = append(contentBlocks, ContentBlock{Type: ContentTypeText, Text: &text})
			}
		} else {
			// Try as array of blocks
			var blocks []mistralContentBlock
			if err := json.Unmarshal(wire.Content, &blocks); err != nil {
				return fmt.Errorf("mistral: unsupported content format")
			}
			for _, b := range blocks {
				switch b.Type {
				case "text":
					if b.Text != nil && *b.Text != "" {
						contentBlocks = append(contentBlocks, ContentBlock{Type: ContentTypeText, Text: b.Text})
					}
				case "image_url":
					// image_url may be string or object {url: string}
					var urlStr string
					if err := json.Unmarshal([]byte(fmt.Sprintf(`"%s"`, b.ImageURL)), &urlStr); err == nil {
						// String format
						contentBlocks = append(contentBlocks, ContentBlock{
							Type:        ContentTypeImage,
							ImageSource: &ImageSource{Type: "url", URL: &urlStr},
						})
					} else {
						// Try object format
						var obj struct{ URL string `json:"url"` }
						imgBytes, _ := json.Marshal(b.ImageURL)
						if err := json.Unmarshal(imgBytes, &obj); err == nil && obj.URL != "" {
							contentBlocks = append(contentBlocks, ContentBlock{
								Type:        ContentTypeImage,
								ImageSource: &ImageSource{Type: "url", URL: &obj.URL},
							})
						} else {
							return fmt.Errorf("mistral: invalid image_url block")
						}
					}
				default:
					return fmt.Errorf("mistral: unsupported content block type %q", b.Type)
				}
			}
		}
	}

	// Tool calls from assistant
	for _, call := range wire.ToolCalls {
		raw := rawJSONFromString(call.Function.Arguments)
		callID := call.ID
		name := call.Function.Name
		contentBlocks = append(contentBlocks, ContentBlock{
			Type:      ContentTypeToolUse,
			ToolUseID: &callID,
			ToolName:  &name,
			ToolInput: raw,
		})
	}

	// Tool result message
	if wire.Role == MessageRoleTool {
		if wire.ToolCallID == "" {
			return fmt.Errorf("mistral: tool message missing tool_call_id")
		}
		raw := rawJSONFromString(rawToString(wire.Content))
		callID := wire.ToolCallID
		contentBlocks = append(contentBlocks, ContentBlock{
			Type:              ContentTypeToolResult,
			ToolResultID:      &callID,
			ToolResultContent: raw,
		})
	}

	mm.Message = Message{
		Role:    wire.Role,
		Content: contentBlocks,
	}
	return nil
}

// rawJSONFromString returns json.RawMessage from a possibly non-JSON string
func rawJSONFromString(s string) json.RawMessage {
	if s == "" {
		return nil
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage([]byte(s))
	}
	quoted, _ := json.Marshal(s)
	return json.RawMessage(quoted)
}

// rawToString converts raw JSON to string best-effort
func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}
