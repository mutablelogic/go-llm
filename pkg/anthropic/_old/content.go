package anthropic

import (
	"encoding/json"
	"strings"

	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Content struct {
	Type string `json:"type"` // image, document, text, tool_use
	ContentText
	ContentAttachment
	*ContentTool
	ContentToolResult
	CacheControl *cachecontrol `json:"cache_control,omitempty"` // ephemeral
}

type ContentText struct {
	Text string `json:"text,omitempty"` // text content
}

type ContentTool struct {
	Id        string         `json:"id,omitempty"`           // tool id
	Name      string         `json:"name,omitempty"`         // tool name
	Input     map[string]any `json:"input"`                  // tool input
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
// GLOBALS

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

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return a content object with text content
func NewTextContent(v string) *Content {
	return &Content{
		Type: "text",
		ContentText: ContentText{
			Text: v,
		},
	}
}

// Return a content object with tool result
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

// Make attachment content
func NewAttachment(attachment *llm.Attachment, ephemeral, citations bool) (*Content, error) {
	// Detect mimetype
	mimetype := attachment.Type()
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
