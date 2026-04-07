package httpclient

import (
	"context"
	"fmt"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns a list of models matching the given request parameters.
func (c *Client) ListModels(ctx context.Context, req schema.ModelListRequest) (*schema.ModelList, error) {
	var response schema.ModelList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("model"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// GetModel retrieves a specific model, optionally scoped to a provider.
func (c *Client) GetModel(ctx context.Context, req schema.GetModelRequest) (*schema.Model, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Name == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}

	// Make the request
	requestOpts := make([]client.RequestOpt, 0, 1)
	if req.Provider != "" {
		requestOpts = append(requestOpts, client.OptPath("model", req.Provider, req.Name))
	} else {
		requestOpts = append(requestOpts, client.OptPath("model", req.Name))
	}

	// Make response
	var response schema.Model
	if err := c.DoWithContext(ctx, client.MethodGet, &response, requestOpts...); err != nil {
		return nil, err
	}

	// Return success
	return &response, nil
}

// DownloadModel downloads a model using the given request.
// When progressFn is non-nil, the request is made as an SSE stream and the
// callback is invoked for progress events.
func (c *Client) DownloadModel(ctx context.Context, req schema.DownloadModelRequest, progressFn opt.ProgressFn) (*schema.Model, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Name == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}

	if progressFn != nil {
		return c.downloadModelStream(ctx, req, progressFn)
	}
	return c.downloadModelJSON(ctx, req)
}

// DeleteModel deletes a specific model and returns the deleted model.
func (c *Client) DeleteModel(ctx context.Context, req schema.DeleteModelRequest) (*schema.Model, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Name == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}

	requestOpts := make([]client.RequestOpt, 0, 1)
	if req.Provider != "" {
		requestOpts = append(requestOpts, client.OptPath("model", req.Provider, req.Name))
	} else {
		requestOpts = append(requestOpts, client.OptPath("model", req.Name))
	}

	var response schema.Model
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, requestOpts...); err != nil {
		return nil, err
	}

	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (c *Client) downloadModelJSON(ctx context.Context, req schema.DownloadModelRequest) (*schema.Model, error) {
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.Model
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("model"), client.OptNoTimeout()); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *Client) downloadModelStream(ctx context.Context, req schema.DownloadModelRequest, progressFn opt.ProgressFn) (*schema.Model, error) {
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response *schema.Model
	var streamErr error

	callback := func(evt client.TextStreamEvent) error {
		switch evt.Event {
		case schema.EventProgress:
			var progress schema.ProgressEvent
			if err := evt.Json(&progress); err != nil {
				return fmt.Errorf("malformed progress event: %w", err)
			}
			progressFn(progress.Status, progress.Percent)
		case schema.EventError:
			var streamError schema.StreamError
			if err := evt.Json(&streamError); err != nil {
				return fmt.Errorf("malformed error event: %w", err)
			}
			streamErr = fmt.Errorf("%s", streamError.Error)
		case schema.EventResult:
			var model schema.Model
			if err := evt.Json(&model); err != nil {
				return fmt.Errorf("malformed result event: %w", err)
			}
			response = &model
		}
		return nil
	}

	var discard struct{}
	if err := c.DoWithContext(ctx, httpReq, &discard,
		client.OptPath("model"),
		client.OptReqHeader("Accept", "text/event-stream"),
		client.OptTextStreamCallback(callback),
		client.OptNoTimeout(),
	); err != nil {
		return nil, err
	}
	if streamErr != nil {
		return nil, streamErr
	}
	if response == nil {
		return nil, fmt.Errorf("no result event received in stream")
	}

	return response, nil
}
