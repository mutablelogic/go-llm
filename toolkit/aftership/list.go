package aftership

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/aftership/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// list is the tool which lists trackings, or couriers, and shows one in full,
// when it's the only one
type list struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*list)(nil)

// listInput is what's listed, and which of it
type listInput struct {
	Type           string `json:"type,omitempty" enum:"tracking,courier" help:"What to list: tracking, for the shipments, which it is when it isn't given, or courier" optional:""`
	ID             string `json:"id,omitempty" help:"Identifier of the only tracking to list, or the code of the only courier, such as dhl" optional:""`
	TrackingNumber string `json:"tracking_number,omitempty" help:"Tracking number, of the trackings to list, or whose couriers are detected, when couriers are listed" optional:""`
	Query          string `json:"q,omitempty" help:"Text in a tracking's number, title or order, or words of a courier's name, such as dhl germany" optional:""`
	Status         string `json:"status,omitempty" enum:"Pending,InfoReceived,InTransit,OutForDelivery,AttemptFail,Delivered,AvailableForPickup,Exception,Expired" help:"Status of the trackings to list" optional:""`
	Courier        string `json:"courier,omitempty" help:"Code of the courier of the trackings to list, such as dhl" optional:""`
	Since          string `json:"since,omitempty" help:"How long ago the trackings to list last changed, at most, such as 1h, 1d or 1w" example:"1d" optional:""`
	Limit          uint64 `json:"limit,omitempty" help:"Most to list, which is 20 when it isn't given" min:"1" max:"100" optional:""`
	Cursor         string `json:"cursor,omitempty" help:"The next cursor, of the trackings listed before, for those after them" optional:""`
}

// listOutput is the trackings, or the couriers, and how many there are
type listOutput struct {
	Count     int        `json:"count"`
	Trackings []tracking `json:"trackings,omitzero"`
	Couriers  []courier  `json:"couriers,omitzero"`
	Cursor    string     `json:"next_cursor,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// listDefault is how many are listed, without a limit
	listDefault = 20

	// listLimit is the most which are listed
	listLimit = 100

	typeTracking = "tracking"
	typeCourier  = "courier"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*list) Name() string {
	return "list"
}

func (*list) Description() string {
	return "Lists the shipments which are tracked, with the status of each, and when it's expected, by its identifier, tracking number, status, courier, text, or how lately it changed. When only one is listed, it's in full, with everything which has happened to it. A shipment which has tracking_stopped isn't checked any more, which happens some days after it last changes. With the type courier, finds couriers instead, of the more than a thousand there are, by words of their name, a tracking number, or their code, and when only one is listed, what a tracking of it needs. One which needs_account only works with the person's account with it, set up in AfterShip, so use one which doesn't."
}

func (*list) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[listInput]()
}

// Run lists the trackings, or the couriers
func (t *list) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req listInput
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	req.ID, req.TrackingNumber, req.Query = strings.TrimSpace(req.ID), strings.TrimSpace(req.TrackingNumber), strings.TrimSpace(req.Query)
	if req.Limit == 0 {
		req.Limit = listDefault
	}
	req.Limit = min(req.Limit, listLimit)
	switch strings.TrimSpace(req.Type) {
	case "", typeTracking:
		return t.trackings(ctx, req)
	case typeCourier:
		return t.couriers(ctx, req)
	}
	return nil, llm.ErrBadParameter.Withf("type %q isn't %s, or %s", req.Type, typeTracking, typeCourier)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// trackings lists the trackings, or the one with the identifier
func (t *list) trackings(ctx context.Context, req listInput) (any, error) {
	since, err := sinceTime(req.Since)
	if err != nil {
		return nil, err
	}

	// The one with the identifier
	if req.ID != "" {
		toolkit.Logf(ctx, "Looking at a shipment")
		one, err := t.client.GetTracking(ctx, req.ID, nil)
		if err != nil {
			return nil, err
		}
		return listOutput{Count: 1, Trackings: []tracking{newTrackingDetails(*one)}}, nil
	}

	// Those which match
	toolkit.Logf(ctx, "Looking at the shipments")
	query := httpclient.ListTrackingsRequest{
		Keyword:         req.Query,
		TrackingNumbers: []string{req.TrackingNumber},
		Slug:            []string{req.Courier},
		Tag:             []httpclient.Tag{httpclient.Tag(req.Status)},
		UpdatedAtMin:    since,
		Cursor:          req.Cursor,
		Limit:           req.Limit,
	}
	response, err := t.client.ListTrackings(ctx, &query)
	if err != nil {
		return nil, err
	}
	result := listOutput{Count: response.Pagination.Total, Trackings: []tracking{}}
	if response.Pagination.HasNextPage {
		result.Cursor = response.Pagination.NextCursor
	}

	// The only one is in full, as it's got
	if len(response.Trackings) == 1 {
		one, err := t.client.GetTracking(ctx, response.Trackings[0].ID, nil)
		if err != nil {
			return nil, err
		}
		result.Trackings = append(result.Trackings, newTrackingDetails(*one))
		return result, nil
	}
	for _, one := range response.Trackings {
		result.Trackings = append(result.Trackings, newTracking(one))
	}
	return result, nil
}

// couriers lists the courier with the code, those which the tracking number
// could be of, or those whose name has the words. AfterShip has more than a
// thousand, any of which can be used, whether it's on in the account, or not.
func (t *list) couriers(ctx context.Context, req listInput) (any, error) {
	var couriers []httpclient.Courier
	var err error
	if req.ID == "" && req.TrackingNumber == "" && req.Query == "" {
		return nil, llm.ErrBadParameter.With("there are more than a thousand couriers, so a q, a tracking_number, or an id, is needed")
	}
	toolkit.Logf(ctx, "Looking for couriers")
	switch {
	case req.ID != "":
		if couriers, err = t.client.ListCouriers(ctx, &httpclient.ListCouriersRequest{Slug: []string{req.ID}}); err == nil && len(couriers) == 0 {
			err = llm.ErrNotFound.Withf("courier %q", req.ID)
		}
	case req.TrackingNumber != "":
		couriers, err = t.client.DetectCouriers(ctx, &httpclient.DetectCouriersRequest{TrackingNumber: req.TrackingNumber})
	default:
		couriers, err = t.client.ListCouriers(ctx, nil)
	}
	if err != nil {
		return nil, err
	}

	// Those whose name has the words, those which need an account last, and
	// otherwise as they were, which is the likeliest first, when they're
	// detected, or else the shortest code first, such as dhl
	if words := words(req.Query); len(words) > 0 && req.ID == "" {
		couriers = slices.DeleteFunc(slices.Clone(couriers), func(one httpclient.Courier) bool {
			return !hasWords(one, words)
		})
		if req.TrackingNumber == "" {
			slices.SortStableFunc(couriers, func(a, b httpclient.Courier) int {
				return cmp.Or(cmp.Compare(len(a.Slug), len(b.Slug)), strings.Compare(a.Slug, b.Slug))
			})
		}
	}
	slices.SortStableFunc(couriers, func(a, b httpclient.Courier) int {
		return cmp.Compare(order(needsAccount(a)), order(needsAccount(b)))
	})

	// The only one is in full
	result := listOutput{Count: len(couriers), Couriers: []courier{}}
	couriers = couriers[:min(uint64(len(couriers)), req.Limit)]
	if len(couriers) == 1 {
		result.Couriers = append(result.Couriers, newCourierDetails(couriers[0]))
		return result, nil
	}
	for _, one := range couriers {
		result.Couriers = append(result.Couriers, newCourier(one))
	}
	return result, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sinceTime returns the time which is the duration ago, such as 1h or 7d, or
// zero, when it's empty, or ErrBadParameter, when it isn't a duration, or is
// less than a second, such as a number without a unit
func sinceTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	duration, err := types.ParseDuration(value)
	if err != nil || duration < time.Second {
		return time.Time{}, llm.ErrBadParameter.Withf("since is %q, which isn't a duration, such as 1h, 1d or 1w", value)
	}
	return time.Now().Add(-duration), nil
}

// words returns the words of the text, in lower case, in which a hyphen is
// between words, as it is in a courier's code
func words(text string) []string {
	return strings.Fields(strings.ReplaceAll(strings.ToLower(text), "-", " "))
}

// hasWords returns whether the courier's code, or names, have every word
func hasWords(c httpclient.Courier, words []string) bool {
	text := strings.ToLower(c.Slug + " " + c.Name + " " + c.OtherName)
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

// order returns 1, which is after 0, when it's true
func order(last bool) int {
	if last {
		return 1
	}
	return 0
}
