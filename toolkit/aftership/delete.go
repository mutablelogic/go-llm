package aftership

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/aftership/httpclient"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// remove is the tool which deletes a tracking
type remove struct {
	toolkit.Base
	client *httpclient.Client
}

var _ llm.Tool = (*remove)(nil)

// removeInput is the tracking
type removeInput struct {
	ID string `json:"id" help:"Identifier of the tracking, from the list"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*remove) Name() string {
	return "delete"
}

func (*remove) Description() string {
	return "Deletes a shipment which is tracked, with what has happened to it, and returns it, as it was. It can't be undone, and tracking it again uses the person's plan again, so only delete one which they ask you to. One which has arrived needn't be deleted."
}

func (*remove) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[removeInput]()
}

// Run deletes the tracking, and returns it, as it was
func (t *remove) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req removeInput
	if err := unmarshal(input, &req); err != nil {
		return nil, err
	}
	if req.ID = strings.TrimSpace(req.ID); req.ID == "" {
		return nil, llm.ErrBadParameter.With("id is required")
	}
	toolkit.Logf(ctx, "Deleting a shipment")
	deleted, err := t.client.DeleteTracking(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return newTracking(*deleted), nil
}
