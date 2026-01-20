package impl

// Packages

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	Type     string `json:"type"`               // text or content
	MimeType string `json:"mimeType,omitempty"` // mime type
	Content  string `json:"text,omitempty"`     // text content
	Data     []byte `json:"data,omitempty"`     // base64 encoded data
}
