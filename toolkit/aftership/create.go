package aftership

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/aftership/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// create is the tool which tracks a shipment
type create struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*create)(nil)

// createInput is the shipment, and what its courier needs
type createInput struct {
	TrackingNumber string `json:"tracking_number" help:"Tracking number, as the person, or the courier, gave it. Never make one up."`
	Courier        string `json:"courier,omitempty" help:"Code of the courier, such as dhl, which is detected when it isn't given" optional:""`
	Title          string `json:"title,omitempty" help:"What's in it, or who it's from, in a few words" optional:""`
	Note           string `json:"note,omitempty" help:"A note" optional:""`
	OrderNumber    string `json:"order_number,omitempty" help:"Number of its order" optional:""`
	neededInput
}

// neededInput is what some couriers need to track a shipment
type neededInput struct {
	DestinationPostalCode string `json:"destination_postal_code,omitempty" help:"Postal code it's to, when the courier needs it" optional:""`
	DestinationState      string `json:"destination_state,omitempty" help:"State it's to, when the courier needs it" optional:""`
	DestinationCountry    string `json:"destination_country,omitempty" help:"Country it's to, of three letters, such as DEU, when the courier needs it" optional:""`
	OriginCountry         string `json:"origin_country,omitempty" help:"Country it's from, of three letters, such as DEU, when the courier needs it" optional:""`
	ShipDate              string `json:"ship_date,omitempty" help:"When it was sent, such as 2026-11-24, when the courier needs it" optional:""`
	AccountNumber         string `json:"account_number,omitempty" help:"The sender's account number, when the courier needs it" optional:""`
	Key                   string `json:"key,omitempty" help:"The key, when the courier needs it" optional:""`
}

// createOutput is the tracking, and whether it was tracked already
type createOutput struct {
	tracking
	Existing bool   `json:"existing,omitempty"`
	Info     string `json:"info,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*create) Name() string {
	return "create"
}

func (*create) Description() string {
	return "Tracks a shipment, by its tracking number, and returns it, or the one which is tracked already. Each new one uses the person's plan, so only track a shipment which they want tracked. Its courier is detected, and some need more than the number, which the couriers' list says, as a required field. Its status is usually only known some minutes later."
}

func (*create) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[createInput]()
}

// Run tracks the shipment, and returns it, or the one which is tracked
// already
func (t *create) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req createInput
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.TrackingNumber = strings.TrimSpace(req.TrackingNumber); req.TrackingNumber == "" {
		return nil, llm.ErrBadParameter.With("tracking_number is required")
	}
	toolkit.Logf(ctx, "Tracking a shipment")
	meta := req.neededInput.meta()
	meta.Slug, meta.Title, meta.Note, meta.OrderNumber = ptr(req.Courier), ptr(req.Title), ptr(req.Note), ptr(req.OrderNumber)
	created, err := t.client.CreateTracking(ctx, &httpclient.CreateTrackingRequest{TrackingNumber: req.TrackingNumber, TrackingMeta: meta})
	if err == nil {
		return createOutput{tracking: newTrackingDetails(*created)}, nil
	} else if !errors.Is(err, llm.ErrConflict) {
		return nil, err
	}

	// The one which is tracked already, when it can be found
	response, listErr := t.client.ListTrackings(ctx, &httpclient.ListTrackingsRequest{TrackingNumbers: []string{req.TrackingNumber}, Slug: []string{req.Courier}, Limit: 1})
	if listErr != nil {
		return nil, errors.Join(err, listErr)
	} else if len(response.Trackings) == 0 {
		return nil, err
	}
	return createOutput{tracking: newTrackingDetails(response.Trackings[0]), Existing: true, Info: "It was tracked already, and wasn't changed."}, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// meta returns what's set, which the courier needs
func (n neededInput) meta() httpclient.TrackingMeta {
	return httpclient.TrackingMeta{
		DestinationPostalCode:    ptr(n.DestinationPostalCode),
		DestinationState:         ptr(n.DestinationState),
		DestinationCountryRegion: ptr(n.DestinationCountry),
		OriginCountryRegion:      ptr(n.OriginCountry),
		TrackingShipDate:         ptr(n.ShipDate),
		TrackingAccountNumber:    ptr(n.AccountNumber),
		TrackingKey:              ptr(n.Key),
	}
}
