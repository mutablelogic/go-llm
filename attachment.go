package llm

import (
	"io"
	"os"
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
// PUBLIC METHODS

func (a *Attachment) Filename() string {
	return a.filename
}

func (a *Attachment) Data() []byte {
	return a.data
}
