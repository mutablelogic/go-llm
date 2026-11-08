package httpclient

import (
	"context"
	"net/http"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	transport "github.com/mutablelogic/go-client/pkg/transport"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client of a resource, at its endpoint, with go-client, whose
// requests which aren't authorized return an AuthError, and which
// discovers how a token for the resource is got
type Client struct {
	*client.Client
	endpoint string
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client of the resource at the endpoint, with go-client's
// options, such as a token, which the endpoint is applied after, so an
// option can't change it
func New(endpoint string, opts ...client.ClientOpt) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	c, err := client.New(append(opts, client.OptEndpoint(endpoint))...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c, endpoint: endpoint}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Endpoint returns the endpoint of the resource
func (c *Client) Endpoint() string {
	return c.endpoint
}

// DoAuthWithContext makes the request, as DoWithContext does, and returns an
// AuthError, with the server's challenge, when it isn't authorized, which
// is a 401 Unauthorized response with a WWW-Authenticate header, so how a
// token is got for it can be discovered, with DiscoverWithError
func (c *Client) DoAuthWithContext(ctx context.Context, req client.Payload, v any, opts ...client.RequestOpt) error {
	var recorder *transport.Recorder
	opts = append(opts, client.OptReqTransport(func(parent http.RoundTripper) http.RoundTripper {
		recorder = transport.NewRecorder(parent)
		return recorder
	}))
	if err := c.Client.DoWithContext(ctx, req, v, opts...); err != nil {
		if recorder != nil && recorder.StatusCode() == http.StatusUnauthorized {
			if authErr := ParseChallenge(recorder.Header()); authErr != nil {
				return authErr
			}
		}
		return err
	}
	return nil
}
