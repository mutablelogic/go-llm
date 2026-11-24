package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// hookTransport replaces tokenPlaceholder, which the path of a request of a
// webhook's URL ends with, with the webhook's token, beneath the client's
// tracing, and logging, so they, and the errors of requests, have the path
// without it
type hookTransport struct {
	base http.RoundTripper
}

// tokenKey is the key of a request's token, in its context
type tokenKey struct{}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// tokenPlaceholder is in the place of the token, in a request's path
	tokenPlaceholder = "{token}"

	// maxErrorBody is the most of a response's body which is read, when it
	// isn't a success
	maxErrorBody = 64 << 10
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// withHookTransport is the option of the transport, which is the last of
// the options, as the last transport is the innermost
func withHookTransport() client.ClientOpt {
	return client.OptTransport(func(base http.RoundTripper) http.RoundTripper {
		if base == nil {
			base = http.DefaultTransport
		}
		return &hookTransport{base: base}
	})
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// RoundTrip sends a copy of the request, whose path has the token, when it
// has one, and returns the response, with the request, rather than its copy.
// A response which isn't a success, whose location, such as a redirect's, or
// body, such as an error's, has the token has the placeholder in its place,
// which is replaced again, when a redirect is followed.
func (t *hookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, ok := req.Context().Value(tokenKey{}).(string)
	prefix, found := strings.CutSuffix(req.URL.Path, "/"+tokenPlaceholder)
	if !ok || !found {
		return t.base.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	clone.URL.Path = prefix + "/" + token
	clone.URL.RawPath = (&url.URL{Path: prefix}).EscapedPath() + "/" + url.PathEscape(token)
	resp, err := t.base.RoundTrip(clone)
	if err != nil {
		return resp, err
	}
	resp.Request = req
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		redact := strings.NewReplacer(url.PathEscape(token), tokenPlaceholder, token, tokenPlaceholder)
		if location := resp.Header.Get("Location"); location != "" {
			resp.Header.Set("Location", redact.Replace(location))
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		body = []byte(redact.Replace(string(body)))
		resp.Body, resp.ContentLength = io.NopCloser(bytes.NewReader(body)), int64(len(body))
		resp.Header.Del("Content-Length")
	}
	return resp, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// withToken returns the context, of a request which has the token
func withToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey{}, token)
}
