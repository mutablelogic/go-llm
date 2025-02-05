package llm

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type AttachmentMeta struct {
	Id        string `json:"id,omitempty"`
	Filename  string `json:"filename"`
	Data      []byte `json:"data"`
	ExpiresAt uint64 `json:"expires_at,omitempty"`
	Caption   string `json:"transcript,omitempty"`
}

// Attachment for messages
type Attachment struct {
	meta AttachmentMeta
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// ReadAttachment returns an attachment from a reader object.
// It is the responsibility of the caller to close the reader.
func ReadAttachment(r io.Reader) (*Attachment, error) {
	var filename string
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if f, ok := r.(*os.File); ok {
		filename = f.Name()
	}
	return &Attachment{
		meta: AttachmentMeta{
			Filename: filename,
			Data:     data,
		},
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (a *Attachment) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &a.meta)
}

func (a *Attachment) MarshalJSON() ([]byte, error) {
	// Create a JSON representation
	var j struct {
		Id       string `json:"id,omitempty"`
		Filename string `json:"filename,omitempty"`
		Type     string `json:"type"`
		Bytes    uint64 `json:"bytes"`
		Caption  string `json:"transcript,omitempty"`
	}
	j.Id = a.meta.Id
	j.Filename = a.meta.Filename
	j.Type = a.Type()
	j.Bytes = uint64(len(a.meta.Data))
	j.Caption = a.meta.Caption
	return json.Marshal(j)
}

func (a *Attachment) String() string {
	data, err := json.MarshalIndent(a.meta, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (a *Attachment) Filename() string {
	return a.meta.Filename
}

func (a *Attachment) Data() []byte {
	return a.meta.Data
}

func (a *Attachment) Caption() string {
	return a.meta.Caption
}

func (a *Attachment) Type() string {
	// Mimetype based on content
	mimetype := http.DetectContentType(a.meta.Data)
	if mimetype == "application/octet-stream" && a.meta.Filename != "" {
		// Detect mimetype from extension
		mimetype = mime.TypeByExtension(filepath.Ext(a.meta.Filename))
	}
	return mimetype
}

func (a *Attachment) Url() string {
	return "data:" + a.Type() + ";base64," + base64.StdEncoding.EncodeToString(a.meta.Data)
}
