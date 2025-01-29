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

// Implementation of a message
type message struct {
	MessageMeta
}

var _ llm.Context = (*message)(nil)

// Message with text or object content
type MessageMeta struct {
	Role    string     `json:"role"`
	Content []*Content `json:"content,omitempty"`
}

type Content struct {
	Type         string           `json:"type"`                    // image, document, text
	Text         string           `json:"text,omitempty"`          // text content
	Title        string           `json:"title,omitempty"`         // title of the document
	Context      string           `json:"context,omitempty"`       // context of the document
	Citations    *contentcitation `json:"citations,omitempty"`     // citations of the document
	Source       *contentsource   `json:"source,omitempty"`        // image or document content
	CacheControl *cachecontrol    `json:"cache_control,omitempty"` // ephemeral
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
// STRINGIFY

func (m message) String() string {
	data, err := json.MarshalIndent(m.MessageMeta, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m message) Role() string {
	return m.MessageMeta.Role
}

// Create user message context
func (*Client) UserPrompt(text string, opts ...llm.Opt) llm.Context {
	// Get attachments
	opt, err := apply(opts...)
	if err != nil {
		return nil
	}

	context := &message{}
	context.MessageMeta.Role = "user"
	context.MessageMeta.Content = make([]*Content, 0, len(opt.data)+1)

	// Append the text
	context.MessageMeta.Content = append(context.MessageMeta.Content, &Content{
		Type: "text",
		Text: text,
	})

	// Append any additional data
	for _, data := range opt.data {
		context.MessageMeta.Content = append(context.MessageMeta.Content, data)
	}

	// Return the context
	return context
}

// Create the result of calling a tool
func (*Client) ToolResult(any) llm.Context {
	return nil
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
