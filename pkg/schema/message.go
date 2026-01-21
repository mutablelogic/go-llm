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

type content struct {
	// If content is a string
	Text *string

	// If content is an array of text
	TextArray []string

	// If content is an array of types
	TypeArray []contentType
}

type contentType struct {
	Type string `json:"type,omitempty"`
	*contentTypeText
	*contentTypeToolResult
	*contentTypeToolUse
	*contentTypeImage
}

type contentTypeText struct {
	Text string `json:"text,omitempty"`
}

type contentTypeToolResult struct {
	ToolUseId string  `json:"tool_use_id,omitempty"`
	Content   content `json:"content,omitempty"`
	IsError   bool    `json:"is_error,omitempty"`
}

type contentTypeToolUse struct {
	Id    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type contentTypeImage struct {
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
	Content content `json:"content,omitempty"` // string or array of text, reference, image_url
	Tokens  uint    `json:"-"`                 // number of tokens for the message
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func StringMessage(role, message string) Message {
	return Message{
		Role:    role,
		Content: content{Text: &message},
	}
}

// ToolResultMessage creates a user message containing a tool result
func ToolResultMessage(toolUseId, result string, isError bool) Message {
	return Message{
		Role: "user",
		Content: content{
			TypeArray: []contentType{{
				Type: "tool_result",
				contentTypeToolResult: &contentTypeToolResult{
					ToolUseId: toolUseId,
					Content:   content{Text: &result},
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
		Content: content{
			TypeArray: []contentType{{
				Type: "image",
				contentTypeImage: &contentTypeImage{
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
// STRINGIFY

func (m Message) String() string {
	return Stringify(m)
}

////////////////////////////////////////////////////////////////////////////////
// JSON

func (c *content) UnmarshalJSON(data []byte) error {
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

func (c content) MarshalJSON() ([]byte, error) {
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
