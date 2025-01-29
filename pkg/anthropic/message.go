package anthropic

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

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
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type Attachment struct {
	Type   string `json:"type"` // image, document
	Source struct {
		Type         string `json:"type"`                    // base64
		MediaType    string `json:"media_type"`              // image/jpeg, image/png, image/gif, image/webp, application/pdf
		Data         []byte `json:"data"`                    // ...base64 encoded data
		CacheControl string `json:"cache_control,omitempty"` // ephemeral
	} `json:"source"`
}

type Text struct {
	Type string `json:"type"` // text
	Text string `json:"text"`
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
	if len(opt.data) > 0 {
		content := make([]any, 0, len(opt.data)+1)
		content = append(content, &Text{
			Type: "text",
			Text: text,
		})
		for _, data := range opt.data {
			content = append(content, data)
		}
		context.MessageMeta.Content = content
	} else {
		context.MessageMeta.Content = text
	}
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
	}
)

// Create a new attachment from an io.Reader
func NewAttachment(r io.Reader) (*Attachment, error) {
	var data bytes.Buffer
	if _, err := io.Copy(&data, r); err != nil {
		return nil, err
	}

	// Detect mimetype
	mimetype := http.DetectContentType(data.Bytes())
	typ, exists := supportedAttachments[mimetype]
	if !exists {
		return nil, llm.ErrBadParameter.Withf("unsupported or undetected mimetype %q", mimetype)
	}

	// Create attachment
	attachment := &Attachment{
		Type: typ,
	}
	attachment.Source.Type = "base64"
	attachment.Source.MediaType = mimetype
	attachment.Source.Data = data.Bytes()

	// Return success
	return attachment, nil
}
