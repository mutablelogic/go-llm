package openai

import "github.com/mutablelogic/go-llm"

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	Type    string         `json:"type"`              // text or content
	Content string         `json:"content,omitempty"` // text content
	Audio   llm.Attachment `json:"audio,omitempty"`   // audio content
}

///////////////////////////////////////////////////////////////////////////////
// LICECYCLE

func NewContentString(typ, content string) *Content {
	return &Content{Type: typ, Content: content}
}
