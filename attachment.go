package llm

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// General attachment metadata
type AttachmentMeta struct {
	Id        string `json:"id,omitempty"`
	Filename  string `json:"filename,omitempty"`
	ExpiresAt uint64 `json:"expires_at,omitempty"`
	Caption   string `json:"transcript,omitempty"`
	Data      []byte `json:"data"`
	Type      string `json:"type"`
}

// OpenAI image metadata
type ImageMeta struct {
	Url    string `json:"url,omitempty"`
	Data   []byte `json:"b64_json,omitempty"`
	Prompt string `json:"revised_prompt,omitempty"`
}

// Attachment for messages
type Attachment struct {
	meta  *AttachmentMeta
	image *ImageMeta
}

const (
	defaultMimetype = "application/octet-stream"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewAttachment creates a new, empty attachment
func NewAttachment() *Attachment {
	return new(Attachment)
}

// NewAttachment with OpenAI image
func NewAttachmentWithImage(image *ImageMeta) *Attachment {
	return &Attachment{image: image}
}

// ReadAttachment returns an attachment from a reader object.
// It is the responsibility of the caller to close the reader.
func ReadAttachment(r io.Reader, mimetype ...string) (*Attachment, error) {
	var filename, typ string
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if f, ok := r.(*os.File); ok {
		filename = f.Name()
	}
	if len(mimetype) > 0 {
		typ = mimetype[0]
	}
	return &Attachment{
		meta: &AttachmentMeta{
			Filename: filename,
			Data:     data,
			Type:     typ,
		},
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

// Convert JSON into an attachment
func (a *Attachment) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &a.meta)
}

// Convert an attachment into JSON
func (a *Attachment) MarshalJSON() ([]byte, error) {
	// Create a JSON representation
	var j struct {
		Id       string `json:"id,omitempty"`
		Filename string `json:"filename,omitempty"`
		Type     string `json:"type"`
		Bytes    uint64 `json:"bytes"`
		Hash     string `json:"hash,omitempty"`
		Caption  string `json:"caption,omitempty"`
	}

	j.Type = a.Type()
	j.Caption = a.Caption()
	j.Hash = a.Hash()
	j.Filename = a.Filename()
	if a.meta != nil {
		j.Id = a.meta.Id
		j.Bytes = uint64(len(a.meta.Data))
	} else if a.image != nil {
		j.Bytes = uint64(len(a.image.Data))
	}

	return json.Marshal(j)
}

// Stringify an attachment
func (a *Attachment) String() string {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Compute and print the MD5 hash
func (a *Attachment) Hash() string {
	hash := md5.New()
	hash.Write(a.Data())
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// Write out attachment
func (a *Attachment) Write(w io.Writer) (int, error) {
	if a.meta != nil {
		return w.Write(a.meta.Data)
	}
	if a.image != nil {
		return w.Write(a.image.Data)
	}
	return 0, io.EOF
}

// Return the filename of an attachment
func (a *Attachment) Filename() string {
	if a.meta != nil && a.meta.Filename != "" {
		return a.meta.Filename
	}
	// Obtain filename from MD5
	if ext, err := mime.ExtensionsByType(a.Type()); err == nil && len(ext) > 0 {
		return a.Hash() + ext[0]
	}
	return ""
}

// Return the raw attachment data
func (a *Attachment) Data() []byte {
	if a.meta != nil {
		return a.meta.Data
	}
	if a.image != nil {
		return a.image.Data
	}
	return nil
}

// Return the caption for the attachment
func (a *Attachment) Caption() string {
	if a.meta != nil {
		return strings.TrimSpace(a.meta.Caption)
	}
	if a.image != nil {
		return strings.TrimSpace(a.image.Prompt)
	}
	return ""
}

// Return the mime media type for the attachment, based
// on the data and/or filename extension. Returns an empty string if
// there is no data or filename
func (a *Attachment) Type() string {
	// If there's a mimetype set, use this
	if a.meta != nil && a.meta.Type != "" {
		return a.meta.Type
	}

	// If there's no data or filename, return empty
	if len(a.Data()) == 0 && a.Filename() == "" {
		return ""
	}

	// Mimetype based on content
	mimetype := defaultMimetype
	if len(a.Data()) > 0 {
		mimetype = http.DetectContentType(a.Data())
		if mimetype != defaultMimetype {
			return mimetype
		}
	}

	// Mimetype based on filename
	if a.meta != nil && a.meta.Filename != "" {
		// Detect mimetype from extension
		mimetype = mime.TypeByExtension(filepath.Ext(a.meta.Filename))
	}

	// Return the default mimetype
	return mimetype
}

func (a *Attachment) Url() string {
	return "data:" + a.Type() + ";base64," + base64.StdEncoding.EncodeToString(a.Data())
}

// Streaming includes the ability to append data
func (a *Attachment) Append(other *Attachment) {
	if a.meta != nil {
		if other.meta.Id != "" {
			a.meta.Id = other.meta.Id
		}
		if other.meta.Filename != "" {
			a.meta.Filename = other.meta.Filename
		}
		if other.meta.ExpiresAt != 0 {
			a.meta.ExpiresAt = other.meta.ExpiresAt
		}
		if other.meta.Caption != "" {
			a.meta.Caption += other.meta.Caption
		}
		if len(other.meta.Data) > 0 {
			a.meta.Data = append(a.meta.Data, other.meta.Data...)
		}
	}
	// TODO: Append for image
}
