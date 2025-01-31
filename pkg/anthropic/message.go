package anthropic

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Message with text or object content
type MessageMeta struct {
	Role    string     `json:"role"`
	Content []*Content `json:"content,omitempty"`
}

type Content struct {
	Type string `json:"type"` // image, document, text, tool_use
	ContentText
	ContentAttachment
	ContentTool
	ContentToolResult
	CacheControl *cachecontrol `json:"cache_control,omitempty"` // ephemeral
}

type ContentText struct {
	Text string `json:"text,omitempty"` // text content
}

type ContentTool struct {
	Id        string         `json:"id,omitempty"`           // tool id
	Name      string         `json:"name,omitempty"`         // tool name
	Input     map[string]any `json:"input,omitempty"`        // tool input
	InputJson string         `json:"partial_json,omitempty"` // partial json input (for streaming)
}

type ContentAttachment struct {
	Title     string           `json:"title,omitempty"`     // title of the document
	Context   string           `json:"context,omitempty"`   // context of the document
	Citations *contentcitation `json:"citations,omitempty"` // citations of the document
	Source    *contentsource   `json:"source,omitempty"`    // image or document content
}

type ContentToolResult struct {
	Id      string     `json:"tool_use_id,omitempty"` // tool id
	Content []*Content `json:"content,omitempty"`
}

type contentsource struct {
	Type      string `json:"type"`       // base64 or text
	MediaType string `json:"media_type"` // image/jpeg, image/png, image/gif, image/webp, application/pdf, text/plain
	Data      any    `json:"data"`       // ...base64 or text encoded data
}

type cachecontrol struct {
	Type string `json:"type"` // ephemeral
}

type contentcitation struct {
	Enabled bool `json:"enabled"` // true
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return a Content object with text content
func NewTextContent(v string) *Content {
	content := new(Content)
	content.Type = "text"
	content.ContentText.Text = v
	return content
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m MessageMeta) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

var (
	supportedAttachments = map[string]string{
		"image/jpeg":      "image",
		"image/png":       "image",
		"image/gif":       "image",
		"image/webp":      "image",
		"application/pdf": "document",
		"text/plain":      "text",
	}
)

// Read content from an io.Reader
func ReadContent(r io.Reader, ephemeral, citations bool) (*Content, error) {
	var data bytes.Buffer
	if _, err := io.Copy(&data, r); err != nil {
		return nil, err
	}

	// Detect mimetype
	mimetype := http.DetectContentType(data.Bytes())
	if strings.HasPrefix(mimetype, "text/") {
		// Switch to text/plain - TODO: charsets?
		mimetype = "text/plain"
	}

	// Check supported mimetype
	typ, exists := supportedAttachments[mimetype]
	if !exists {
		return nil, llm.ErrBadParameter.Withf("unsupported or undetected mimetype %q", mimetype)
	}

	// Create attachment
	attachment := new(Content)
	attachment.Type = typ
	if ephemeral {
		attachment.CacheControl = &cachecontrol{Type: "ephemeral"}
	}

	// Handle by type
	switch typ {
	case "text":
		attachment.Type = "document"
		attachment.Source = &contentsource{
			Type:      "text",
			MediaType: mimetype,
			Data:      data.String(),
		}

		// Check for filename
		if f, ok := r.(*os.File); ok && f.Name() != "" {
			attachment.Title = f.Name()
		}

		// Check for citations
		if citations {
			attachment.Citations = &contentcitation{Enabled: true}
		}
	case "document":
		// Check for filename
		if f, ok := r.(*os.File); ok && f.Name() != "" {
			attachment.Title = f.Name()
		}

		// Check for citations
		if citations {
			attachment.Citations = &contentcitation{Enabled: true}
		}
		attachment.Source = &contentsource{
			Type:      "base64",
			MediaType: mimetype,
			Data:      data.Bytes(),
		}
	case "image":
		attachment.Source = &contentsource{
			Type:      "base64",
			MediaType: mimetype,
			Data:      data.Bytes(),
		}
	default:
		return nil, llm.ErrBadParameter.Withf("unsupported attachment type %q", typ)
	}

	// Return success
	return attachment, nil
}
