package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
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

// BatchResult represents a single result from a batch
type BatchResult struct {
	CustomId string             `json:"custom_id"`
	Result   BatchResultContent `json:"result"`
}

// BatchResultContent represents the result content which varies by type
type BatchResultContent struct {
	Type    string            `json:"type"` // "succeeded", "errored", "canceled", "expired"
	Message *messagesResponse `json:"message,omitempty"`
	Error   *BatchError       `json:"error,omitempty"`
}

// BatchError represents an error in a batch result
type BatchError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
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

// GetBatchResults retrieves the results of a completed batch as a slice of BatchResult.
// The batch must be in "ended" status for results to be available.
func (anthropic *Client) GetBatchResults(ctx context.Context, id string) ([]BatchResult, error) {
	// Get the batch first to check status and get results URL
	batch, err := anthropic.GetBatch(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check if batch has ended
	if batch.Status != "ended" {
		return nil, llm.ErrConflict.Withf("batch is not ended, current status: %s", batch.Status)
	}

	// Check if results URL is available
	if batch.ResultsUrl == nil || *batch.ResultsUrl == "" {
		return nil, llm.ErrNotFound.With("batch results URL not available")
	}

	// Fetch results from the results URL
	return anthropic.fetchBatchResults(ctx, *batch.ResultsUrl)
}

// fetchBatchResults fetches and parses JSONL results from a URL
func (anthropic *Client) fetchBatchResults(ctx context.Context, resultsUrl string) ([]BatchResult, error) {
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resultsUrl, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	req.Header.Set("anthropic-version", defaultVersion)
	req.Header.Set("x-api-key", anthropic.apiKey)

	// Execute request using standard http client
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		return nil, llm.ErrInternalServerError.Withf("unexpected status: %s", resp.Status)
	}

	// Parse JSONL response
	return parseJSONL(resp.Body)
}

// parseJSONL parses a JSONL stream into a slice of BatchResult
func parseJSONL(r io.Reader) ([]BatchResult, error) {
	var results []BatchResult
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var result BatchResult
		if err := json.Unmarshal(line, &result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
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

func (b BatchResult) String() string {
	return schema.Stringify(b)
}
