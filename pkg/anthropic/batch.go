package anthropic

import (
	"context"
	"net/http"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type batchRequest struct {
	Id     string      `json:"custom_id"`
	Params batchParams `json:"params,omitempty"`
}

type batchParams struct {
	MaxTokens uint            `json:"max_tokens,omitempty"`
	Model     string          `json:"model"`
	Messages  *schema.Session `json:"messages,omitempty"`
}

type Batch struct {
	Id                string     `json:"id"`
	Type              string     `json:"type,omitempty"`    // "message_batch"
	Status            string     `json:"processing_status"` // "in_progress" or "canceling" or "ended"
	CreatedAt         time.Time  `json:"created_at"`
	ArchivedAt        *time.Time `json:"archived_at,omitempty"`
	CancelInitiatedAt *time.Time `json:"cancel_initiated_at,omitempty"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	RequestCounts     struct {
		Processing uint `json:"processing"`
		Succeeded  uint `json:"succeeded"`
		Errored    uint `json:"errored"`
		Canceled   uint `json:"canceled"`
		Expired    uint `json:"expired"`
	} `json:"request_counts"`
	ResultsUrl *string `json:"results_url,omitempty"`
}

// BatchList represents the response from listing batches
type BatchList struct {
	Data    []Batch `json:"data"`
	HasMore bool    `json:"has_more"`
	FirstId string  `json:"first_id"`
	LastId  string  `json:"last_id"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create a batch messages request with a specific model and messages
func (anthropic *Client) CreateBatch(ctx context.Context, id, model string, messages *schema.Session) (*Batch, error) {
	// Create a request
	type req struct {
		Requests []batchRequest `json:"requests"`
	}
	request, err := client.NewJSONRequest(req{
		Requests: []batchRequest{
			{
				Id: id,
				Params: batchParams{
					Model:    model,
					Messages: messages,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Send the request
	var response Batch
	if err := anthropic.DoWithContext(ctx, request, &response, client.OptPath("messages", "batches")); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// Get a batch by ID
func (anthropic *Client) GetBatch(ctx context.Context, id string) (*Batch, error) {
	var response Batch
	if err := anthropic.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("messages", "batches", id)); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// List batches with optional pagination
func (anthropic *Client) ListBatches(ctx context.Context, opts ...opt.Opt) (*BatchList, error) {
	var response BatchList
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	if err := anthropic.DoWithContext(ctx, client.MethodGet, &response,
		client.OptPath("messages", "batches"),
		client.OptQuery(o.Query("after_id", "before_id", "limit")),
	); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// Cancel a batch by ID
func (anthropic *Client) CancelBatch(ctx context.Context, id string) (*Batch, error) {
	var response Batch
	if err := anthropic.DoWithContext(ctx, client.NewRequestEx(http.MethodPost, ""), &response, client.OptPath("messages", "batches", id, "cancel")); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// Delete a batch by ID
func (anthropic *Client) DeleteBatch(ctx context.Context, id string) error {
	var response struct {
		Id   string `json:"id"`
		Type string `json:"type,omitempty"`
	}
	if err := anthropic.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("messages", "batches", id)); err != nil {
		return err
	} else if response.Id != id {
		return llm.ErrInternalServerError.With("unexpected response deleting batch")
	} else if response.Type != "message_batch_deleted" {
		return llm.ErrInternalServerError.With("unexpected response deleting batch")
	}

	// Return success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (b Batch) String() string {
	return schema.Stringify(b)
}

func (b BatchList) String() string {
	return schema.Stringify(b)
}
