package openai

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	Type    string `json:"type"`              // text or content
	Content string `json:"content,omitempty"` // text content
}

///////////////////////////////////////////////////////////////////////////////
// LICECYCLE

func NewContentString(typ, content string) *Content {
	return &Content{Type: typ, Content: content}
}
