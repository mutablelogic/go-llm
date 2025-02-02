package mistral

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Message with text or object content
type MessageMeta struct {
	Role    string `json:"role"` // assistant, user, tool, system
	Prefix  bool   `json:"prefix,omitempty"`
	Content any    `json:"content,omitempty"`
	// ContentTools
}

type Content struct {
	Type        string                       `json:"type"` // text, reference, image_url
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
// LIFECYCLE

// Return a Content object with text content
func NewContent(t, v, p string) *Content {
	content := new(Content)
	content.Type = t
	if v != "" {
		content.Text = (*Text)(&v)
	}
	if p != "" {
		content.Prediction = (*Prediction)(&p)
	}
	return content
}

// Return a Content object with text content
func NewTextContent(v string) *Content {
	return NewContent("text", v, "")
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return the text content
func (m MessageMeta) Text() string {
	if text, ok := m.Content.(string); ok {
		return text
	}
	return ""
}

/*
	if arr, ok := m.Content.([]Content); ok {
	if len(m.Content) == 0 {
		return ""
	}
	var text []string
	for _, content := range m.Content {
		if content.Type == "text" && content.Text != nil {
			text = append(text, string(*content.Text))
		}
	}
	return strings.Join(text, "\n")
}
*/
