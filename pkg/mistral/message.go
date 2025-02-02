package mistral

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Possible completions
type Completions []Completion

// Completion Variation
type Completion struct {
	Index   uint64  `json:"index"`
	Message Message `json:"message"`
	Reason  string  `json:"finish_reason,omitempty"`
}

// Message with text or object content
type Message struct {
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

// Return a Content object with text content (either in "text" or "prediction" field)
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

// Return the number of completions
func (c Completions) Num() int {
	return len(c)
}

// Return the role of the completion
func (c Completions) Role() string {
	// The role should be the same for all completions, let's use the first one
	if len(c) == 0 {
		return ""
	}
	return c[0].Message.Role
}

// Return the text content for a specific completion
func (c Completions) Text(index int) string {
	if index < 0 || index >= len(c) {
		return ""
	}
	completion := c[index].Message
	if text, ok := completion.Content.(string); ok {
		return text
	}
	// Will the text be in other forms?
	return ""
}
