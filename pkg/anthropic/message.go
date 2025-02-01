package anthropic

import (
	"encoding/json"
	"net/http"
	"strings"

	// Packages
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
	Id      string `json:"tool_use_id,omitempty"` // tool id
	Content any    `json:"content,omitempty"`
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

// Return a Content object with tool result
func NewToolResultContent(v llm.ToolResult) *Content {
	content := new(Content)
	content.Type = "tool_result"
	content.ContentToolResult.Id = v.Call().Id()
	//	content.ContentToolResult.Name = v.Call().Name()

	// We only support JSON encoding for the moment
	data, err := json.Marshal(v.Value())
	if err != nil {
		content.ContentToolResult.Content = err.Error()
	} else {
		content.ContentToolResult.Content = string(data)
	}

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
func attachmentContent(attachment *llm.Attachment, ephemeral, citations bool) (*Content, error) {
	// Detect mimetype
	mimetype := http.DetectContentType(attachment.Data())
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
	content := new(Content)
	content.Type = typ
	if ephemeral {
		content.CacheControl = &cachecontrol{Type: "ephemeral"}
	}

	// Handle by type
	switch typ {
	case "text":
		content.Type = "document"
		content.Title = attachment.Filename()
		content.Source = &contentsource{
			Type:      "text",
			MediaType: mimetype,
			Data:      string(attachment.Data()),
		}
		if citations {
			content.Citations = &contentcitation{Enabled: true}
		}
	case "document":
		content.Source = &contentsource{
			Type:      "base64",
			MediaType: mimetype,
			Data:      attachment.Data(),
		}
		if citations {
			content.Citations = &contentcitation{Enabled: true}
		}
	case "image":
		content.Source = &contentsource{
			Type:      "base64",
			MediaType: mimetype,
			Data:      attachment.Data(),
		}
	default:
		return nil, llm.ErrBadParameter.Withf("unsupported attachment type %q", typ)
	}

	// Return success
	return content, nil
}
