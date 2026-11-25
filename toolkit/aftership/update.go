package aftership

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/aftership/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// update is the tool which changes a tracking, or completes it
type update struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*update)(nil)

// updateInput is the tracking, what's changed of it, and why it's completed
type updateInput struct {
	ID          string `json:"id" help:"Identifier of the tracking, from the list"`
	Courier     string `json:"courier,omitempty" help:"Code of the courier, such as dhl, when it's wrong" optional:""`
	Title       string `json:"title,omitempty" help:"What's in it, or who it's from, in a few words" optional:""`
	Note        string `json:"note,omitempty" help:"A note" optional:""`
	OrderNumber string `json:"order_number,omitempty" help:"Number of its order" optional:""`
	neededInput
	Completed   string `json:"completed,omitempty" enum:"delivered,lost,returned" help:"Completes it, so it's no longer tracked, as it was delivered, lost, or returned to its sender" optional:""`
	CompletedAt string `json:"completed_at,omitempty" help:"When it was delivered, lost or returned, such as 2026-11-24, or now, when it isn't given" optional:""`
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// reasons are why a tracking is completed, as AfterShip says them
var reasons = map[string]string{
	"delivered": httpclient.ReasonDelivered,
	"lost":      httpclient.ReasonLost,
	"returned":  httpclient.ReasonReturnedToSender,
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*update) Name() string {
	return "update"
}

func (*update) Description() string {
	return "Changes a shipment which is tracked, such as its title, or its courier, and what isn't given stays as it was. It can also complete it, as delivered, lost, or returned, so it's no longer tracked, which can't be undone. Only complete it when the person says so, as the courier usually does."
}

func (*update) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[updateInput]()
}

// Run changes the tracking, completes it, or both, and returns it
func (t *update) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req updateInput
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.ID = strings.TrimSpace(req.ID); req.ID == "" {
		return nil, llm.ErrBadParameter.With("id is required")
	}
	meta := req.neededInput.meta()
	meta.Slug, meta.Title, meta.Note, meta.OrderNumber = ptr(req.Courier), ptr(req.Title), ptr(req.Note), ptr(req.OrderNumber)
	changed := !reflect.ValueOf(meta).IsZero()
	completed := strings.ToLower(strings.TrimSpace(req.Completed))
	reason, ok := reasons[completed]
	if completed != "" && !ok {
		return nil, llm.ErrBadParameter.Withf("completed %q isn't delivered, lost or returned", req.Completed)
	} else if completed == "" && strings.TrimSpace(req.CompletedAt) != "" {
		return nil, llm.ErrBadParameter.With("completed_at needs completed")
	} else if !changed && completed == "" {
		return nil, llm.ErrBadParameter.With("nothing to change")
	}

	// What's changed, and then completed, as it can't be changed once it is
	var result *httpclient.Tracking
	var err error
	if changed {
		toolkit.Logf(ctx, "Changing a shipment")
		if result, err = t.client.UpdateTracking(ctx, req.ID, &httpclient.UpdateTrackingRequest{TrackingMeta: meta}); err != nil {
			return nil, err
		}
	}
	if completed != "" {
		toolkit.Logf(ctx, "Completing a shipment, as %s", completed)
		if result, err = t.client.CompleteTracking(ctx, req.ID, &httpclient.CompleteTrackingRequest{Reason: reason, Time: strings.TrimSpace(req.CompletedAt)}); err != nil && changed {
			return nil, fmt.Errorf("it was changed, but not completed: %w", err)
		} else if err != nil {
			return nil, err
		}
	}
	return newTrackingDetails(*result), nil
}
