/*
httpclient implements an API client for AfterShip Tracking
https://www.aftership.com/docs/tracking
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

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Client struct {
	*client.Client
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Version is the version of the API
	Version = "2026-07"

	endPoint = "https://api.aftership.com/tracking/" + Version

	// headerKey is the header of the API key
	headerKey = "as-api-key"

	// requestsPerSecond is AfterShip's rate limit, by default
	requestsPerSecond = 10

	// maxErrorBody is the most of an error's response which is read
	maxErrorBody = 64 << 10

	// codeExists is the code of the error that a tracking exists already,
	// whose status is 400
	codeExists = 4003
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns a client for AfterShip Tracking with the API key. The endpoint
// can be replaced with client.OptEndpoint, for example for testing, and the
// rate limit, which is AfterShip's, with client.OptRateLimit.
func New(apiKey string, opts ...client.ClientOpt) (*Client, error) {
	// Check for missing API key
	if apiKey = strings.TrimSpace(apiKey); apiKey == "" {
		return nil, llm.ErrBadParameter.With("missing API key")
	}

	// Create client, with the default endpoint, and rate limit, first so they
	// can be replaced, and the API key in a header
	opts = append([]client.ClientOpt{client.OptEndpoint(endPoint), client.OptRateLimit(requestsPerSecond)}, opts...)
	opts = append(opts, client.OptHeader(headerKey, apiKey))
	client, err := client.New(opts...)
	if err != nil {
		return nil, err
	}

	// Return the client
	return &Client{Client: client}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - TRACKINGS

// ListTrackings returns a page of the trackings matching the request, and how
// many there are. The next page is got with the cursor of its pagination.
func (c *Client) ListTrackings(ctx context.Context, req *ListTrackingsRequest) (*TrackingList, error) {
	return do[TrackingList](ctx, c, nil, req.Values(), "trackings")
}

// CreateTracking adds a tracking, whose courier is detected, when the request
// has none, and returns it. Returns ErrConflict when there's a tracking of the
// number already.
func (c *Client) CreateTracking(ctx context.Context, req *CreateTrackingRequest) (*Tracking, error) {
	if req == nil || strings.TrimSpace(req.TrackingNumber) == "" {
		return nil, llm.ErrBadParameter.With("missing tracking number")
	}
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}
	return do[Tracking](ctx, c, payload, nil, "trackings")
}

// GetTracking returns the tracking with the identifier, with its checkpoints.
// Returns ErrNotFound when there's none.
func (c *Client) GetTracking(ctx context.Context, id string, req *GetTrackingRequest) (*Tracking, error) {
	if id = strings.TrimSpace(id); id == "" {
		return nil, errMissingID
	}
	return do[Tracking](ctx, c, nil, req.Values(), "trackings", id)
}

// UpdateTracking changes what's set of the tracking with the identifier, and
// returns it. Returns ErrNotFound when there's none.
func (c *Client) UpdateTracking(ctx context.Context, id string, req *UpdateTrackingRequest) (*Tracking, error) {
	if id = strings.TrimSpace(id); id == "" {
		return nil, errMissingID
	} else if req == nil {
		return nil, llm.ErrBadParameter.With("missing change")
	}
	payload, err := client.NewJSONRequestEx(http.MethodPut, req, types.ContentTypeJSON)
	if err != nil {
		return nil, err
	}
	return do[Tracking](ctx, c, payload, nil, "trackings", id)
}

// DeleteTracking deletes the tracking with the identifier, and returns it, as
// it was. Returns ErrNotFound when there's none.
func (c *Client) DeleteTracking(ctx context.Context, id string) (*Tracking, error) {
	if id = strings.TrimSpace(id); id == "" {
		return nil, errMissingID
	}
	return do[Tracking](ctx, c, client.NewRequestEx(http.MethodDelete, types.ContentTypeJSON), nil, "trackings", id)
}

// RetrackTracking tracks the tracking with the identifier again, which has
// expired, and which can only be done once, and returns it
func (c *Client) RetrackTracking(ctx context.Context, id string) (*Tracking, error) {
	if id = strings.TrimSpace(id); id == "" {
		return nil, errMissingID
	}
	return do[Tracking](ctx, c, client.NewRequestEx(http.MethodPost, types.ContentTypeJSON), nil, "trackings", id, "retrack")
}

// CompleteTracking marks the tracking with the identifier as completed, such
// as delivered, so it's no longer tracked, and returns it
func (c *Client) CompleteTracking(ctx context.Context, id string, req *CompleteTrackingRequest) (*Tracking, error) {
	if id = strings.TrimSpace(id); id == "" {
		return nil, errMissingID
	} else if req == nil || req.Reason == "" {
		return nil, llm.ErrBadParameter.With("missing reason")
	}
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}
	return do[Tracking](ctx, c, payload, nil, "trackings", id, "mark-as-completed")
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - COURIERS

// ListCouriers returns the couriers matching the request
func (c *Client) ListCouriers(ctx context.Context, req *ListCouriersRequest) ([]Courier, error) {
	response, err := do[courierList](ctx, c, nil, req.Values(), "couriers")
	if err != nil {
		return nil, err
	}
	return response.Couriers, nil
}

// DetectCouriers returns the couriers which could have the tracking number,
// the likeliest first
func (c *Client) DetectCouriers(ctx context.Context, req *DetectCouriersRequest) ([]Courier, error) {
	if req == nil || strings.TrimSpace(req.TrackingNumber) == "" {
		return nil, llm.ErrBadParameter.With("missing tracking number")
	}
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}
	response, err := do[courierList](ctx, c, payload, nil, "couriers", "detect")
	if err != nil {
		return nil, err
	}
	return response.Couriers, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// errMissingID is the error of a tracking without its identifier, whose path
// would be that of every tracking
var errMissingID = llm.ErrBadParameter.With("missing tracking id")

// do sends the payload, which is a GET when it's nil, with the query, to the
// path, and returns the data of the response, or its error. Each request says
// it's JSON, as AfterShip's own clients do, whether it has a payload, or not.
func do[T any](ctx context.Context, c *Client, payload client.Payload, query url.Values, path ...any) (*T, error) {
	opts := []client.RequestOpt{client.OptPath(path...), client.OptReqHeader(types.ContentTypeHeader, types.ContentTypeJSON), client.OptResponse(responseErr)}
	if len(query) > 0 {
		opts = append(opts, client.OptQuery(query))
	}
	var response struct {
		Data T `json:"data"`
	}
	if err := c.DoWithContext(ctx, payload, &response, opts...); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// responseErr returns the error of the response, as AfterShip says it, or nil
// when it isn't an error, or doesn't say, so the error is of its status
func responseErr(response *http.Response) error {
	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		return nil
	}

	// What the response says, which can be read again
	data, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	if err != nil {
		return err
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	var body struct {
		Meta struct {
			Code    int    `json:"code"`
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"meta"`
	}
	if json.Unmarshal(data, &body) != nil || body.Meta.Message == "" {
		return nil
	}

	// The error of the status, apart from a tracking which exists already
	kind := llm.ErrBadParameter
	switch status := response.StatusCode; {
	case body.Meta.Code == codeExists:
		kind = llm.ErrConflict
	case status == http.StatusNotFound:
		kind = llm.ErrNotFound
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		kind = llm.ErrForbidden
	case status == http.StatusTooManyRequests, status >= 500:
		kind = llm.ErrServiceUnavailable
	}
	return kind.Withf("%s (%d)", body.Meta.Message, body.Meta.Code)
}
