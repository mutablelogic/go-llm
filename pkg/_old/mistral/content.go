package mistral

import (
	"net/url"

	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	Type        string                       `json:"type"` // text or content
	*Text       `json:"text,omitempty"`      // text content
	*Prediction `json:"content,omitempty"`   // prediction
	*Image      `json:"image_url,omitempty"` // image_url
}

// text content
type Text string

// text content
type Prediction string

// either a URL or "data:image/png;base64," followed by the base64 encoded image
type Image string

///////////////////////////////////////////////////////////////////////////////
// LICECYCLE

func NewPrediction(content Prediction) *Content {
	return &Content{Type: "content", Prediction: &content}
}

func NewTextContext(text Text) *Content {
	return &Content{Type: "text", Text: &text}
}

func NewImageData(image *llm.Attachment) *Content {
	url := Image(image.Url())
	return &Content{Type: "image_url", Image: &url}
}

func NewImageUrl(u *url.URL) *Content {
	url := Image(u.String())
	return &Content{Type: "image_url", Image: &url}
}
