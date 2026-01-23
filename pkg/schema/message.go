package schema

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	// If content is a string
	Text *string

	// If content is an array of text
	TextArray []string

	// If content is an array of types
	TypeArray []contentType
}

type contentType struct {
	Type string `json:"type,omitempty"`
	*ContentTypeText
	*ContentTypeToolResult
	*ContentTypeToolUse
	*ContentTypeImage
}

type ContentTypeText struct {
	Text string `json:"text,omitempty"`
}

type ContentTypeToolResult struct {
	ToolUseId string  `json:"tool_use_id,omitempty"`
	Content   Content `json:"content,omitempty"`
	IsError   bool    `json:"is_error,omitempty"`
}

type ContentTypeToolUse struct {
	Id    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type ContentTypeImage struct {
	Source *imageSource `json:"source,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/png", "image/jpeg", etc.
	Data      string `json:"data"`       // base64-encoded data
}

// Represents content to send to or received from an LLM
type Message struct {
	Role    string  `json:"role,omitempty"`    // assistant, user, tool, system
	Content Content `json:"content,omitempty"` // string or array of text, reference, image_url
	Tokens  uint    `json:"-"`                 // number of tokens for the message
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func StringMessage(role, message string) Message {
	return Message{
		Role:    role,
		Content: Content{Text: &message},
	}
}

// ToolResultMessage creates a user message containing a tool result
func ToolResultMessage(toolUseId, result string, isError bool) Message {
	return Message{
		Role: "user",
		Content: Content{
			TypeArray: []contentType{{
				Type: "tool_result",
				ContentTypeToolResult: &ContentTypeToolResult{
					ToolUseId: toolUseId,
					Content:   Content{Text: &result},
					IsError:   isError,
				},
			}},
		},
	}
}

// ImageMessage creates a user message containing an image.
// The image data is read immediately and base64-encoded.
// If mediaType is empty, it will be detected from the image data.
// Returns an error if the data cannot be read or is not a valid image type.
func ImageMessage(r io.Reader, mediaType string) (Message, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Message{}, err
	}

	// Detect media type if not provided
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}

	// Validate it's an image
	if !strings.HasPrefix(mediaType, "image/") {
		return Message{}, errors.New("invalid image type: " + mediaType)
	}

	return Message{
		Role: "user",
		Content: Content{
			TypeArray: []contentType{{
				Type: "image",
				ContentTypeImage: &ContentTypeImage{
					Source: &imageSource{
						Type:      "base64",
						MediaType: mediaType,
						Data:      base64.StdEncoding.EncodeToString(data),
					},
				},
			}},
		},
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Text returns the text content of the message, concatenating all text parts
func (m Message) Text() string {
	// If content is a simple string
	if m.Content.Text != nil {
		return *m.Content.Text
	}

	// If content is an array of strings
	if len(m.Content.TextArray) > 0 {
		return strings.Join(m.Content.TextArray, "\n")
	}

	// If content is an array of types, extract text from each
	var texts []string
	for _, ct := range m.Content.TypeArray {
		if ct.ContentTypeText != nil && ct.ContentTypeText.Text != "" {
			texts = append(texts, ct.ContentTypeText.Text)
		}
	}
	if len(texts) > 0 {
		return strings.Join(texts, "\n")
	}

	return ""
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Message) String() string {
	return Stringify(m)
}

////////////////////////////////////////////////////////////////////////////////
// JSON

func (c *Content) UnmarshalJSON(data []byte) error {
	// Try string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.Text = &str
		return nil
	}

	// Try array of strings
	var strArray []string
	if err := json.Unmarshal(data, &strArray); err == nil {
		c.TextArray = strArray
		return nil
	}

	// Try array of types
	var typeArray []contentType
	if err := json.Unmarshal(data, &typeArray); err == nil {
		c.TypeArray = typeArray
		return nil
	}

	return nil
}

func (c Content) MarshalJSON() ([]byte, error) {
	if c.Text != nil {
		return json.Marshal(c.Text)
	}
	if c.TextArray != nil {
		return json.Marshal(c.TextArray)
	}
	if c.TypeArray != nil {
		return json.Marshal(c.TypeArray)
	}
	return json.Marshal(nil)
}
