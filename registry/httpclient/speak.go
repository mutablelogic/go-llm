package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// speech writes the audio of a response, as it's read, and keeps its type,
// and what's said of it, in its X-Meta header
type speech struct {
	w           io.Writer
	contentType string
	meta        schema.SpeakMeta
}

var _ client.Unmarshaler = (*speech)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Speak writes the request's text, spoken by its model, which is found by
// its name, or the provider and name separated by ":", such as
// "ollama:phonon:latest", as audio, to the writer, as it's read, and returns
// its type, such as audio/wav, and the model which spoke it, and the usage.
// The request isn't timed out, as speaking a long text can take a while.
func (c *Client) Speak(ctx context.Context, req schema.SpeakRequest, w io.Writer) (string, *schema.SpeakMeta, error) {
	if strings.TrimSpace(req.Model) == "" {
		return "", nil, llm.ErrBadParameter.With("missing model")
	} else if strings.TrimSpace(req.Text) == "" {
		return "", nil, llm.ErrBadParameter.With("missing text")
	} else if w == nil {
		return "", nil, llm.ErrBadParameter.With("missing writer")
	}
	payload, err := client.NewJSONRequestEx(http.MethodPost, req, types.ContentTypeAny)
	if err != nil {
		return "", nil, err
	}
	out := &speech{w: w}
	if err := c.DoWithContext(ctx, payload, out, client.OptPath("llm", "speak"), client.OptNoTimeout()); err != nil {
		return "", nil, err
	}
	return out.contentType, &out.meta, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Unmarshal writes the audio of the response, and keeps its type, and what's
// said of it, which is none, when its header can't be read
func (s *speech) Unmarshal(header http.Header, r io.Reader) error {
	s.contentType = header.Get(types.ContentTypeHeader)
	if meta := header.Get("X-Meta"); meta != "" {
		_ = json.Unmarshal([]byte(meta), &s.meta)
	}
	_, err := io.Copy(s.w, r)
	return err
}
