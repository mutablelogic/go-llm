package impl

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	Type     string `json:"type"`               // text or content
	MimeType string `json:"mimeType,omitempty"` // mime type
	Content  string `json:"text,omitempty"`     // text content
	Data     []byte `json:"data,omitempty"`     // base64 encoded data
}

///////////////////////////////////////////////////////////////////////////////
// LICECYCLE

func FromValue(value any) (*Content, error) {
	switch value := value.(type) {
	case string:
		return &Content{Type: "text", Content: value}, nil
	case llm.Attachment:
		mimetype := value.Type()
		switch mimetype {
		case "image/png", "image/jpeg", "image/gif":
			return &Content{Type: "image", MimeType: value.Type(), Data: value.Data()}, nil
		case "audio/wav", "audio/mpeg":
			return &Content{Type: "audio", MimeType: value.Type(), Data: value.Data()}, nil
		default:
			return nil, llm.ErrBadParameter.Withf("Unsupported attachment type %q", mimetype)
		}
	default:
		return nil, llm.ErrBadParameter.Withf("Unsupported return type %T", value)
	}
}
