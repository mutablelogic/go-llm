package schema

import (
	"encoding/json"
	"fmt"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// OllamaMessage wraps a universal Message for Ollama-specific JSON marshaling
type OllamaMessage struct {
	Message
}

///////////////////////////////////////////////////////////////////////////////
// MARSHALING

// MarshalJSON converts the universal Message to Ollama's JSON format
func (om OllamaMessage) MarshalJSON() ([]byte, error) {
	// Ollama format: { "role": "user", "content": "text", "images": ["base64..."] }

	// Collect text and images from content blocks
	var textParts []string
	var images []string

	for _, block := range om.Content {
		switch block.Type {
		case "text":
			if block.Text != nil && *block.Text != "" {
				textParts = append(textParts, *block.Text)
			}

		case "image":
			if block.ImageSource != nil {
				if block.ImageSource.Type != "base64" {
					return nil, fmt.Errorf("ollama only supports base64-encoded images, not %s", block.ImageSource.Type)
				}
				if block.ImageSource.Data != nil {
					images = append(images, *block.ImageSource.Data)
				}
			}

		case "document":
			// Ollama doesn't support PDF or binary documents
			if block.DocumentSource != nil {
				if block.DocumentSource.Type == "text" && block.DocumentSource.Text != nil {
					// Allow text documents by extracting the text
					textParts = append(textParts, *block.DocumentSource.Text)
				} else {
					return nil, fmt.Errorf("ollama does not support document type %q (only text documents are supported)", block.DocumentSource.Type)
				}
			}

		case "tool_use", "tool_result":
			return nil, fmt.Errorf("ollama does not support tool calls (type: %s)", block.Type)

		case "thinking":
			return nil, fmt.Errorf("ollama does not support thinking blocks")

		default:
			if block.Type != "" {
				return nil, fmt.Errorf("ollama does not support content type: %s", block.Type)
			}
		}
	}

	// Combine all text parts
	content := ""
	if len(textParts) > 0 {
		content = textParts[0]
		for i := 1; i < len(textParts); i++ {
			content += "\n\n" + textParts[i]
		}
	}

	// Build the Ollama message structure
	ollamaMsg := struct {
		Role    string   `json:"role"`
		Content string   `json:"content"`
		Images  []string `json:"images,omitempty"`
	}{
		Role:    om.Role,
		Content: content,
		Images:  images,
	}

	return json.Marshal(ollamaMsg)
}

// UnmarshalJSON converts Ollama's JSON format to the universal Message
func (om *OllamaMessage) UnmarshalJSON(data []byte) error {
	// Unmarshal into Ollama-specific structure
	var ollamaMsg struct {
		Role    string   `json:"role"`
		Content string   `json:"content"`
		Images  []string `json:"images,omitempty"`
	}

	if err := json.Unmarshal(data, &ollamaMsg); err != nil {
		return err
	}

	// Convert to universal format
	universalContent := make([]ContentBlock, 0)

	// Add text content
	if ollamaMsg.Content != "" {
		universalContent = append(universalContent, ContentBlock{
			Type: "text",
			Text: &ollamaMsg.Content,
		})
	}

	// Add images
	for _, imageData := range ollamaMsg.Images {
		data := imageData
		universalContent = append(universalContent, ContentBlock{
			Type: "image",
			ImageSource: &ImageSource{
				Type:      "base64",
				MediaType: "image/jpeg", // Ollama doesn't specify media type
				Data:      &data,
			},
		})
	}

	om.Message = Message{
		Role:    ollamaMsg.Role,
		Content: universalContent,
	}

	return nil
}
