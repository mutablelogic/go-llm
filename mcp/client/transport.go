package client

import (
	"net/http"
	"net/url"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// sameOrigin is a transport which sends only the requests to the origin,
// which is the scheme, host and port, of the server's URL, so the
// credentials, such as a bearer token, and headers, which the transport it
// wraps adds, aren't sent to another, such as by a redirect, or an SSE
// message endpoint, which the server says
type sameOrigin struct {
	origin string
	next   http.RoundTripper
}

var _ http.RoundTripper = (*sameOrigin)(nil)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// httpClient returns the HTTP client of the MCP SDK's requests, which is a
// copy of go-client's, with its transports, such as its user agent, headers,
// token, and tracing, which only send requests to the server's origin, and
// which isn't redirected to another, but without its timeout, as the
// streams of an MCP session last as long as it does, and are cancelled with
// its context
func (c *Client) httpClient() *http.Client {
	httpClient := *c.Client.Client
	httpClient.Timeout = 0
	origin := originOf(c.endpoint)
	next := httpClient.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	httpClient.Transport = &sameOrigin{origin: origin, next: next}
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if originOf(req.URL.String()) != origin {
			return llm.ErrBadParameter.Withf("%s redirected to another origin, %s", c.endpoint, req.URL.Redacted())
		} else if len(via) >= 10 {
			return llm.ErrBadParameter.Withf("%s redirected too many times", c.endpoint)
		}
		return nil
	}
	return &httpClient
}

// transports returns the MCP SDK's transports of the server, in the order
// they're tried, as it's connected: streamable HTTP, of the 2025-03-26
// protocol, and then SSE, of the 2024-11-05 protocol, for a server which
// hasn't it, which share an HTTP client
func (c *Client) transports() []sdkmcp.Transport {
	httpClient := c.httpClient()
	return []sdkmcp.Transport{
		&sdkmcp.StreamableClientTransport{Endpoint: c.endpoint, HTTPClient: httpClient},
		&sdkmcp.SSEClientTransport{Endpoint: c.endpoint, HTTPClient: httpClient},
	}
}

// RoundTrip sends the request, when it's to the origin, before the
// transport it wraps adds its credentials, and headers, or returns an error
func (t *sameOrigin) RoundTrip(req *http.Request) (*http.Response, error) {
	if originOf(req.URL.String()) != t.origin {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, llm.ErrBadParameter.Withf("request to %s isn't to the server's origin, %s", req.URL.Redacted(), t.origin)
	}
	return t.next.RoundTrip(req)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// originOf returns the origin of the URL, which is its scheme, host and
// port, with the scheme's default port, when it has none, in lower case,
// or nothing, when it isn't a URL
func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" {
		switch scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return scheme + "://" + strings.ToLower(u.Hostname()) + ":" + port
}
