package httpclient

import (
	"context"
	"net/http"
	"net/url"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// queryTransport adds the query of a request's context to its URL, as it's
// sent, so what's asked for, such as where someone is, isn't in the URL which
// is logged, traced, or in an error
type queryTransport struct {
	http.RoundTripper
}

// queryKey is the key of a request's query, in its context
type queryKey struct{}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func newQueryTransport(parent http.RoundTripper) http.RoundTripper {
	if parent == nil {
		parent = http.DefaultTransport
	}
	return &queryTransport{RoundTripper: parent}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// RoundTrip sends a copy of the request, with its query, and returns the
// response, with the request, rather than its copy. A redirect isn't followed,
// as its location would have the query, and can be another server's.
func (t *queryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	query, ok := req.Context().Value(queryKey{}).(url.Values)
	if !ok {
		return t.RoundTripper.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	clone.URL.RawQuery = query.Encode()
	resp, err := t.RoundTripper.RoundTrip(clone)
	if err != nil {
		return nil, err
	}
	resp.Request = req
	if location, err := resp.Location(); err == nil && resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		resp.Body.Close()
		return nil, llm.ErrServiceUnavailable.Withf("the server redirects to %s://%s%s, so use its endpoint", location.Scheme, location.Host, location.EscapedPath())
	}
	return resp, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// withQuery returns the context of a request with the query
func withQuery(ctx context.Context, query url.Values) context.Context {
	return context.WithValue(ctx, queryKey{}, query)
}
