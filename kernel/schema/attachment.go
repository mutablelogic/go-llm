package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Attachment represents binary or URI-referenced media (images, documents, etc.)
type Attachment struct {
	ContentType string   `json:"type" help:"Attachment MIME type, for example image/png or application/pdf" example:"image/png"`
	Data        []byte   `json:"data,omitempty" help:"Inline attachment payload encoded as a byte string" example:"iVBORw0KGgo="`
	URL         *url.URL `json:"url,omitempty" help:"Attachment URL reference, for example https, gs, or file" example:"https://example.com/image.png"`
}

func (a Attachment) MarshalJSON() ([]byte, error) {
	type attachmentJSON struct {
		ContentType string `json:"type"`
		Data        []byte `json:"data,omitempty"`
		URL         string `json:"url,omitempty"`
	}

	out := attachmentJSON{
		ContentType: a.ContentType,
		Data:        a.Data,
	}
	if a.URL != nil {
		out.URL = a.URL.String()
	}

	return json.Marshal(out)
}

func (a *Attachment) UnmarshalJSON(data []byte) error {
	type attachmentJSON struct {
		ContentType string          `json:"type"`
		Data        []byte          `json:"data,omitempty"`
		URL         json.RawMessage `json:"url,omitempty"`
	}

	var in attachmentJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}

	a.ContentType = in.ContentType
	a.Data = in.Data

	parsed, err := unmarshalAttachmentURL(in.URL)
	if err != nil {
		return err
	}
	a.URL = parsed

	return nil
}

func unmarshalAttachmentURL(data json.RawMessage) (*url.URL, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}

	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		if raw == "" {
			return nil, nil
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid attachment url %q: %w", raw, err)
		}
		return parsed, nil
	}

	// Accept the legacy object encoding for backward compatibility.
	var parsed url.URL
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("invalid attachment url: %w", err)
	}
	if parsed.String() == "" {
		return nil, nil
	}
	return &parsed, nil
}

// IsText returns true if the attachment has a text/* MIME type (e.g. text/plain,
// text/html, text/csv). Handles MIME parameters like charset gracefully.
// Such attachments can be converted to text blocks when providers don't
// support them as media uploads.
func (a Attachment) IsText() bool {
	mediaType, _, err := mime.ParseMediaType(a.ContentType)
	if err != nil {
		return strings.HasPrefix(a.ContentType, "text/")
	}
	return strings.HasPrefix(mediaType, "text/")
}

// TextContent returns the attachment's data as a string, optionally prefixed
// with the filename and content type for context. Only meaningful when
// IsText() returns true.
func (a Attachment) TextContent() string {
	text := string(a.Data)
	var header string
	if a.URL != nil && a.URL.Path != "" {
		header += "File: " + a.URL.Path + "\n"
	}
	if a.ContentType != "" {
		header += "Content-Type: " + a.ContentType + "\n"
	}
	if header != "" {
		return header + "\n" + text
	}
	return text
}

// URI returns the URL of the attachment as a string, or an empty string if
// the attachment is inline data only. Satisfies llm.Resource.
func (a Attachment) URI() string {
	if a.URL != nil {
		return a.URL.String()
	}
	return ""
}

// Name returns the last path segment of the attachment URL, or an empty
// string for inline data. Satisfies llm.Resource.
func (a Attachment) Name() string {
	if a.URL != nil && a.URL.Path != "" {
		return path.Base(a.URL.Path)
	}
	return ""
}

// Description returns an empty string. Satisfies llm.Resource.
func (a Attachment) Description() string { return "" }

// Type returns the MIME type of the attachment. Satisfies llm.Resource.
func (a Attachment) Type() string { return a.ContentType }

// maxAttachmentBytes caps the amount of data read from a remote URL to
// prevent unbounded memory use when fetching large responses.
const maxAttachmentBytes = 32 * 1024 * 1024 // 32 MiB

// Read returns a function which reads the attachment's content. Inline data
// is read as it is. Otherwise the URL, when it has a supported scheme (http,
// https or file), is fetched or opened when the function is called, and
// closed when it returns, and at most maxAttachmentBytes of a response are
// read. Satisfies llm.Resource.
func (a Attachment) Read(ctx context.Context) (llm.ReadFunc, error) {
	if len(a.Data) > 0 || a.URL == nil {
		return llm.ReadBytes(a.Data), nil
	}
	switch a.URL.Scheme {
	case "http", "https":
		return func(ctx context.Context, fn func(io.Reader) error) error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL.String(), nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("attachment: HTTP %d fetching %s", resp.StatusCode, a.URL)
			}
			return fn(io.LimitReader(resp.Body, maxAttachmentBytes))
		}, nil
	case "file":
		return func(ctx context.Context, fn func(io.Reader) error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			f, err := os.Open(a.URL.Path)
			if err != nil {
				return err
			}
			defer f.Close()
			return fn(f)
		}, nil
	default:
		return nil, fmt.Errorf("attachment: unsupported URL scheme %q", a.URL.Scheme)
	}
}
