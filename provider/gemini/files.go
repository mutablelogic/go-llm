package gemini

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// upload is the content of a file, which is uploaded, of its type, as
// it's read
type upload struct {
	io.Reader
	contentType string
}

// uploadURL is where a file's content is uploaded to, which the API says
// when its upload starts
type uploadURL string

// uploaded is a file, once it's uploaded
type uploaded struct {
	File file `json:"file"`
}

var _ client.Payload = (*upload)(nil)
var _ client.Unmarshaler = (*uploadURL)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// fileInterval is how often a file which is being processed is checked,
// until it can be used
var fileInterval = time.Second

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// uploadFile uploads the content, of its length, and type, with the Files
// API, as it's read, so it isn't held in memory, and returns the file, once
// it can be used, which is deleted after 48 hours, unless it's deleted
// before
func (c *Client) uploadFile(ctx context.Context, read llm.ReadFunc, length int64, contentType string) (*file, error) {
	// Start the upload, which says where its content is uploaded to
	start, err := client.NewJSONRequest(map[string]any{"file": map[string]any{"displayName": "audio"}})
	if err != nil {
		return nil, err
	}
	var to uploadURL
	if err := c.DoWithContext(ctx, start, &to,
		client.OptReqEndpoint(c.uploadEndpoint()),
		client.OptReqHeader("X-Goog-Upload-Protocol", "resumable"),
		client.OptReqHeader("X-Goog-Upload-Command", "start"),
		client.OptReqHeader("X-Goog-Upload-Header-Content-Length", strconv.FormatInt(length, 10)),
		client.OptReqHeader("X-Goog-Upload-Header-Content-Type", contentType),
	); err != nil {
		return nil, err
	} else if to == "" {
		return nil, llm.ErrServiceUnavailable.With("the upload has nowhere to go")
	}

	// Upload its content, as it's read, and finish
	var result uploaded
	if err := read(ctx, func(r io.Reader) error {
		return c.DoWithContext(ctx, &upload{Reader: r, contentType: contentType}, &result,
			client.OptReqEndpoint(string(to)),
			client.OptReqHeader("X-Goog-Upload-Offset", "0"),
			client.OptReqHeader("X-Goog-Upload-Command", "upload, finalize"),
			client.OptReqTransport(withLength(length)),
			client.OptNoTimeout(),
		)
	}); err != nil {
		return nil, err
	}

	// Wait until it can be used
	f := result.File
	for f.State == "PROCESSING" {
		select {
		case <-ctx.Done():
			c.deleteFile(&f)
			return nil, ctx.Err()
		case <-time.After(fileInterval):
		}
		if err := c.DoWithContext(ctx, client.MethodGet, &f, client.OptPath(segments(f.Name)...)); err != nil {
			c.deleteFile(&f)
			return nil, err
		}
	}
	if f.State == "FAILED" || f.URI == "" {
		c.deleteFile(&f)
		return nil, llm.ErrServiceUnavailable.With("the file couldn't be processed")
	}
	return &f, nil
}

// deleteFile deletes the file, even when the context it was used in is
// cancelled, and ignores an error, as it's deleted after 48 hours anyway
func (c *Client) deleteFile(f *file) {
	if f == nil || f.Name == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = c.DoWithContext(ctx, client.MethodDelete, nil, client.OptPath(segments(f.Name)...))
}

// segments returns the segments of a name, such as files/abc, so its
// slashes are a path's, rather than escaped
func segments(name string) []any {
	var result []any
	for _, segment := range strings.Split(name, "/") {
		result = append(result, segment)
	}
	return result
}

// uploadEndpoint returns where files are uploaded to, which is the API's
// endpoint, under /upload, such as
// https://generativelanguage.googleapis.com/upload/v1beta/files
func (c *Client) uploadEndpoint() string {
	u := *c.url
	u.Path = "/upload/" + strings.Trim(u.Path, "/") + "/files"
	return u.String()
}

// withLength returns a transport which sends the request's body with its
// length, which the upload needs, rather than in chunks
func withLength(length int64) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripper(func(req *http.Request) (*http.Response, error) {
			req.ContentLength = length
			return next.RoundTrip(req)
		})
	}
}

// roundTripper is a function which is a transport
type roundTripper func(*http.Request) (*http.Response, error)

func (fn roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

///////////////////////////////////////////////////////////////////////////////
// PAYLOAD AND RESPONSE

func (*upload) Method() string { return http.MethodPost }
func (*upload) Accept() string { return types.ContentTypeJSON }
func (u *upload) Type() string { return u.contentType }

// Unmarshal reads where the content is uploaded to, from the response's
// header
func (u *uploadURL) Unmarshal(header http.Header, r io.Reader) error {
	*u = uploadURL(header.Get("X-Goog-Upload-URL"))
	_, err := io.Copy(io.Discard, r)
	return err
}
