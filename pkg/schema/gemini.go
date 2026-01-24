package schema

import (
	"encoding/json"
	"fmt"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// GeminiMessage wraps a universal Message for Gemini-specific JSON marshaling
type GeminiMessage struct {
	Message
}

// geminiPart represents Gemini's JSON format for message parts
type geminiPart struct {
	Text string `json:"text,omitempty"`

	InlineData *InlineData `json:"inline_data,omitempty"`
	FileData   *FileData   `json:"file_data,omitempty"`

	// Gemini uses camelCase; keep snake_case for backward/defensive parsing.
	FunctionCall         *FunctionCall   `json:"functionCall,omitempty"`
	FunctionCallFallback *FunctionCall   `json:"function_call,omitempty"`
	FunctionResponse     json.RawMessage `json:"functionResponse,omitempty"`
	FunctionResponseAlt  json.RawMessage `json:"function_response,omitempty"`
}

// InlineData represents embedded image/video data
type InlineData struct {
	MimeType    string  `json:"mime_type"`
	Data        string  `json:"data"` // base64
	DisplayName *string `json:"display_name,omitempty"`
}

// FileData represents file reference
type FileData struct {
	MimeType string `json:"mime_type"`
	FileURI  string `json:"file_uri"`
}

// FunctionCall represents a function call from the model
type FunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

///////////////////////////////////////////////////////////////////////////////
// MARSHALING

// MarshalJSON converts the universal Message to Gemini's JSON format
func (gm GeminiMessage) MarshalJSON() ([]byte, error) {
	// Create Gemini-specific structure
	geminiParts := make([]geminiPart, 0, len(gm.Content))

	for _, block := range gm.Content {
		part := geminiPart{}

		switch block.Type {
		case "text":
			if block.Text != nil {
				part.Text = *block.Text
			}

		case "image":
			if block.ImageSource != nil {
				switch block.ImageSource.Type {
				case "base64":
					if block.ImageSource.Data != nil {
						part.InlineData = &InlineData{
							MimeType:    block.ImageSource.MediaType,
							Data:        *block.ImageSource.Data,
							DisplayName: block.ImageSource.DisplayName,
						}
					}
				case "url", "file":
					// Gemini uses fileUri for both URL and file references
					uri := ""
					if block.ImageSource.URL != nil {
						uri = *block.ImageSource.URL
					} else if block.ImageSource.FileURI != nil {
						uri = *block.ImageSource.FileURI
					}
					if uri != "" {
						part.FileData = &FileData{
							MimeType: block.ImageSource.MediaType,
							FileURI:  uri,
						}
					}
				}
			}

		case "document", "video", "audio":
			// Gemini uses inline_data for base64, file_data for URLs
			if block.DocumentSource != nil {
				switch block.DocumentSource.Type {
				case "base64":
					if block.DocumentSource.Data != nil {
						part.InlineData = &InlineData{
							MimeType: block.DocumentSource.MediaType,
							Data:     *block.DocumentSource.Data,
						}
					}
				case "url":
					if block.DocumentSource.URL != nil {
						part.FileData = &FileData{
							MimeType: block.DocumentSource.MediaType,
							FileURI:  *block.DocumentSource.URL,
						}
					}
				}
			}

		case "tool_use":
			// Gemini calls this functionCall
			if block.ToolUse.ToolName != nil {
				part.FunctionCall = &FunctionCall{
					Name: *block.ToolUse.ToolName,
					Args: block.ToolUse.ToolInput,
				}
			}

		case "tool_result":
			// Gemini calls this functionResponse. Prefer explicit FunctionResponse,
			// else synthesize from ToolResult content/name.
			switch {
			case len(block.FunctionResponse) > 0:
				part.FunctionResponse = block.FunctionResponse
			default:
				name := ""
				if block.ToolResult.ToolName != nil {
					name = *block.ToolResult.ToolName
				} else if block.ToolResult.ToolResultID != nil {
					name = *block.ToolResult.ToolResultID
				}

				// Build Gemini-style functionResponse shape if we have a name.
				if name != "" {
					resp := map[string]interface{}{
						"name": name,
						"response": map[string]interface{}{
							"name":    name,
							"content": []map[string]string{{"text": string(block.ToolResultContent)}},
						},
					}
					if block.ToolError != nil && *block.ToolError {
						resp["response"].(map[string]interface{})["is_error"] = true
					}
					if raw, err := json.Marshal(resp); err == nil {
						part.FunctionResponse = raw
					}
				} else if len(block.ToolResultContent) > 0 {
					// Last resort: drop in raw content
					part.FunctionResponse = block.ToolResultContent
				}
			}
		}

		geminiParts = append(geminiParts, part)
	}

	// Map "assistant" to "model" for Gemini
	role := gm.Role
	if role == "assistant" {
		role = "model"
	}

	// Marshal with Gemini structure
	return json.Marshal(struct {
		Role  string       `json:"role"`
		Parts []geminiPart `json:"parts"`
	}{
		Role:  role,
		Parts: geminiParts,
	})
}

// UnmarshalJSON converts Gemini's JSON format to the universal Message
func (gm *GeminiMessage) UnmarshalJSON(data []byte) error {
	// Unmarshal into Gemini-specific structure
	var geminiMsg struct {
		Role  string       `json:"role"`
		Parts []geminiPart `json:"parts"`
	}

	if err := json.Unmarshal(data, &geminiMsg); err != nil {
		return fmt.Errorf("invalid gemini message format: %w", err)
	}

	// Convert to universal format
	universalContent := make([]ContentBlock, 0, len(geminiMsg.Parts))

	for _, part := range geminiMsg.Parts {
		block := ContentBlock{}

		// Determine block type based on which field is set
		if part.Text != "" {
			block.Type = "text"
			block.Text = &part.Text

		} else if part.InlineData != nil {
			block.Type = "image"
			block.ImageSource = &ImageSource{
				Type:        "base64",
				MediaType:   part.InlineData.MimeType,
				Data:        &part.InlineData.Data,
				DisplayName: part.InlineData.DisplayName,
			}

		} else if part.FileData != nil {
			// Determine type based on mime type
			mimeType := part.FileData.MimeType
			if len(mimeType) >= 5 && mimeType[:5] == "image" {
				block.Type = "image"
				block.ImageSource = &ImageSource{
					Type:      "file",
					MediaType: mimeType,
					FileURI:   &part.FileData.FileURI,
				}
			} else if len(mimeType) >= 5 && mimeType[:5] == "video" {
				block.Type = "video"
				block.DocumentSource = &DocumentSource{
					Type:      "url",
					MediaType: mimeType,
					URL:       &part.FileData.FileURI,
				}
			} else if len(mimeType) >= 5 && mimeType[:5] == "audio" {
				block.Type = "audio"
				block.DocumentSource = &DocumentSource{
					Type:      "url",
					MediaType: mimeType,
					URL:       &part.FileData.FileURI,
				}
			} else {
				// Default to document for PDFs and other files
				block.Type = "document"
				block.DocumentSource = &DocumentSource{
					Type:      "url",
					MediaType: mimeType,
					URL:       &part.FileData.FileURI,
				}
			}

		} else {
			fc := part.FunctionCall
			if fc == nil {
				fc = part.FunctionCallFallback
			}
			if fc != nil {
				block.Type = "tool_use"
				block.ToolUse.ToolName = &fc.Name
				block.ToolUse.ToolInput = fc.Args
				universalContent = append(universalContent, block)
				continue
			}

			fr := part.FunctionResponse
			if len(fr) == 0 {
				fr = part.FunctionResponseAlt
			}
			if len(fr) > 0 {
				block.Type = "tool_result"
				block.FunctionResponse = fr
				universalContent = append(universalContent, block)
				continue
			}
		}

		universalContent = append(universalContent, block)
	}

	// Map "model" to "assistant" for universal format
	role := geminiMsg.Role
	if role == "model" {
		role = "assistant"
	}

	gm.Message = Message{
		Role:    role,
		Content: universalContent,
	}

	return nil
}
