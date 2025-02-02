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

// Attachment for messages
type Attachment struct {
	filename string
	data     []byte
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
	return &Attachment{filename: filename, data: data}, nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (a *Attachment) String() string {
	var j struct {
		Filename string `json:"filename"`
		Type     string `json:"type"`
		Bytes    uint64 `json:"bytes"`
	}
	j.Filename = a.filename
	j.Type = a.Type()
	j.Bytes = uint64(len(a.data))
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (a *Attachment) Filename() string {
	return a.filename
}

func (a *Attachment) Data() []byte {
	return a.data
}

func (a *Attachment) Type() string {
	// Mimetype based on content
	mimetype := http.DetectContentType(a.data)
	if mimetype == "application/octet-stream" && a.filename != "" {
		// Detect mimetype from extension
		mimetype = mime.TypeByExtension(filepath.Ext(a.filename))
	}
	return mimetype
}

func (a *Attachment) Url() string {
	return "data:" + a.Type() + ";base64," + base64.StdEncoding.EncodeToString(a.data)
}
