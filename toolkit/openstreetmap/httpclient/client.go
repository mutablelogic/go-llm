/*
httpclient implements an API client for Nominatim, which finds the places of
OpenStreetMap, by their names, addresses or coordinates
https://nominatim.org/release-docs/latest/api/Overview/

The public server, PublicEndpoint, has a usage policy, which whoever uses it
has to follow, and which can have an application blocked:
https://operations.osmfoundation.org/policies/nominatim/

In short, it's for moderate use, which a person triggers: at most one request
a second, of every user of the application together, with a User-Agent which
says what the application is, and results which are kept, rather than asked
for again. It isn't for autocompletion, for searching systematically, such as
of a grid, or of every place in an area, for geocoding in bulk, or for
personal data, such as exactly where someone is. Data is © OpenStreetMap
contributors, under the ODbL, which has to be said where it's shown:
https://www.openstreetmap.org/copyright
*/
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Client is a client for a Nominatim server, which is best shared, as its rate
// limit, and the results it keeps, are its own
type Client struct {
	*client.Client
	cache *cache
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// PublicEndpoint is the public server, whose usage policy has to be
	// followed. It isn't the default, so using it is a decision, and another
	// server can be used in its place, as the policy needs.
	PublicEndpoint = "https://nominatim.openstreetmap.org"

	// Attribution is who the data is of, which has to be said where it's shown
	Attribution = "© OpenStreetMap contributors, https://www.openstreetmap.org/copyright"

	// userAgent is the application, when it isn't set, as the public server
	// refuses the User-Agent of a library
	userAgent = "go-llm (+https://github.com/mutablelogic/go-llm)"

	// requestsPerSecond is the most requests a second of the public server
	requestsPerSecond = 1

	// cacheSize is the most results which are kept, and cacheAge how long for
	cacheSize = 500
	cacheAge  = 24 * time.Hour

	// maxErrorBody is the most of an error's response which is read
	maxErrorBody = 64 << 10
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client for the Nominatim server at the endpoint, such as
// PublicEndpoint, whose usage policy has to be followed, or one's own. It makes
// at most one request a second, which client.OptRateLimit replaces, for one's
// own server, and says it's go-llm, which client.OptUserAgent replaces, with
// what the application is. Results are kept for a day, so they aren't asked for
// again. What's asked for, such as where someone is, isn't in the client's log,
// its traces, nor its errors.
func New(endpoint string, opts ...client.ClientOpt) (*Client, error) {
	if endpoint = strings.TrimSpace(endpoint); endpoint == "" {
		return nil, llm.ErrBadParameter.With("missing endpoint of a Nominatim server")
	}

	// The defaults are first, so they can be replaced, and the query is added
	// last, within whatever logs, or traces, the request
	opts = append([]client.ClientOpt{client.OptUserAgent(userAgent), client.OptRateLimit(requestsPerSecond)}, opts...)
	opts = append(opts, client.OptEndpoint(endpoint), client.OptTransport(newQueryTransport))
	c, err := client.New(opts...)
	if err != nil {
		return nil, err
	}
	return &Client{Client: c, cache: newCache(cacheSize, cacheAge)}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Search returns the places which match the request, the best first, or none
func (c *Client) Search(ctx context.Context, req *SearchRequest) ([]Place, error) {
	query, err := req.Values()
	if err != nil {
		return nil, err
	}
	var places []Place
	if err := c.get(ctx, "search", query, &places); err != nil {
		return nil, err
	}
	return places, nil
}

// Reverse returns the place which is nearest to the coordinates, of the detail
// of the request's zoom, such as a city, or ErrNotFound, when there's none,
// such as in the sea
func (c *Client) Reverse(ctx context.Context, req *ReverseRequest) (*Place, error) {
	query, err := req.Values()
	if err != nil {
		return nil, err
	}
	var response struct {
		Place
		Error json.RawMessage `json:"error"`
	}
	if err := c.get(ctx, "reverse", query, &response); err != nil {
		return nil, err
	} else if len(response.Error) > 0 {
		return nil, llm.ErrNotFound.With("no place is near the coordinates")
	}
	return &response.Place, nil
}

// Lookup returns the places of the OpenStreetMap objects, such as one which was
// found before, without those which there aren't
func (c *Client) Lookup(ctx context.Context, req *LookupRequest) ([]Place, error) {
	query, err := req.Values()
	if err != nil {
		return nil, err
	}
	var places []Place
	if err := c.get(ctx, "lookup", query, &places); err != nil {
		return nil, err
	}
	return places, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// get decodes the response of the path, with the query, which is the one which
// was kept, when there is one
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	key := path + "?" + query.Encode()
	data, exists := c.cache.get(key)
	if !exists {
		if err := c.DoWithContext(withQuery(ctx, query), nil, &data, client.OptPath(path), client.OptReqHeader("Accept", types.ContentTypeJSON), client.OptResponse(responseErr)); err != nil {
			return err
		}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return llm.ErrServiceUnavailable.Withf("the response isn't Nominatim's: %v", err)
	}
	if !exists {
		c.cache.set(key, data)
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// responseErr returns the error of the response, with what Nominatim says it
// is, when it does, or nil when it isn't an error
func responseErr(response *http.Response) error {
	status := response.StatusCode
	if status >= 200 && status <= 299 {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	if err != nil {
		return err
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	message := http.StatusText(status)
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &body) == nil && body.Error.Message != "" {
		message = body.Error.Message
	}

	// The error of the status
	switch {
	case status == http.StatusNotFound:
		return llm.ErrNotFound.With(message)
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return llm.ErrForbidden.Withf("%s, such as when the server's usage policy isn't followed", message)
	case status == http.StatusTooManyRequests, status >= 500:
		return llm.ErrServiceUnavailable.With(message)
	case status >= 400:
		return llm.ErrBadParameter.With(message)
	}
	return nil
}
