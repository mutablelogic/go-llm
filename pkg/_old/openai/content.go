package openai

import (
	"net/url"

	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	Type    string          `json:"type"`                // text or content
	Content string          `json:"content,omitempty"`   // content content ;-)
	Text    string          `json:"text,omitempty"`      // text content
	Audio   *llm.Attachment `json:"audio,omitempty"`     // audio content
	Image   *Image          `json:"image_url,omitempty"` // image content
}

// A set of tool calls
type ToolCallArray []ToolCall

// text content
type Text string

// text content
type Prediction string

///////////////////////////////////////////////////////////////////////////////
// LICECYCLE

func NewContentString(typ, content string) *Content {
	return &Content{Type: typ, Content: content}
}

func NewTextContext(content string) *Content {
	return &Content{Type: "text", Text: content}
}

func NewImageData(image *llm.Attachment) *Content {
	return &Content{Type: "image_url", Image: &Image{Url: image.Url()}}
}

func NewImageUrl(url *url.URL) *Content {
	return &Content{Type: "image_url", Image: &Image{Url: url.String()}}
}
