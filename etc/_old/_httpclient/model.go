package httpclient

import (
	"context"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns a list of all available models.
// Use WithLimit, WithOffset and WithProvider to paginate and filter results.
func (c *Client) ListModels(ctx context.Context, opts ...opt.Opt) (*schema.ModelList, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create request
	req := client.NewRequest()
	reqOpts := []client.RequestOpt{client.OptPath("model")}
	if q := o.Query(opt.ProviderKey, opt.LimitKey, opt.OffsetKey); len(q) > 0 {
		reqOpts = append(reqOpts, client.OptQuery(q))
	}

	// Perform request
	var response schema.ModelList
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// GetModel retrieves a specific model by name, optionally scoped to a provider.
// If provider is empty, the model is looked up across all providers.
func (c *Client) GetModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	if name == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}

	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Build path: /model/{provider}/{name} or /model/{name}
	req := client.NewRequest()
	var reqOpts []client.RequestOpt
	if provider := o.GetString(opt.ProviderKey); provider != "" {
		reqOpts = append(reqOpts, client.OptPath("model", provider, name))
	} else {
		reqOpts = append(reqOpts, client.OptPath("model", name))
	}

	// Perform request
	var response schema.Model
	if err := c.DoWithContext(ctx, req, &response, reqOpts...); err != nil {
		return nil, err
	}

	// Return the response
	return &response, nil
}

// DownloadModel downloads a model by name. Use WithProvider to target a specific
// provider. If a progress callback is set via opt.WithProgress, the download
// streams SSE progress events and calls the callback for each one.
func (c *Client) DownloadModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	if name == "" {
		return nil, fmt.Errorf("model name cannot be empty")
	}

	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	req := schema.DownloadModelRequest{
		Provider: o.GetString(opt.ProviderKey),
		Name:     name,
	}

	if progressFn := o.GetProgress(); progressFn != nil {
		return c.downloadModelStream(ctx, req, progressFn)
	}
	return c.downloadModelJSON(ctx, req)
}

// DeleteModel deletes a model by name. Use WithProvider to target a specific provider.
func (c *Client) DeleteModel(ctx context.Context, name string, opts ...opt.Opt) error {
	if name == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return err
	}

	// Build path: model/{provider}/{name} or model/{name}
	var reqOpts []client.RequestOpt
	if provider := o.GetString(opt.ProviderKey); provider != "" {
		reqOpts = append(reqOpts, client.OptPath("model", provider, name))
	} else {
		reqOpts = append(reqOpts, client.OptPath("model", name))
	}

	return c.DoWithContext(ctx, client.MethodDelete, nil, reqOpts...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (c *Client) downloadModelJSON(ctx context.Context, req schema.DownloadModelRequest) (*schema.Model, error) {
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response schema.Model
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("model"), client.OptNoTimeout()); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) downloadModelStream(ctx context.Context, req schema.DownloadModelRequest, progressFn opt.ProgressFn) (*schema.Model, error) {
	payload, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	var response *schema.Model
	var streamErr error

	callback := func(evt client.TextStreamEvent) error {
		switch evt.Event {
		case schema.EventProgress:
			var p schema.ProgressEvent
			if err := evt.Json(&p); err != nil {
				return fmt.Errorf("malformed progress event: %w", err)
			}
			progressFn(p.Status, p.Percent)
		case schema.EventError:
			var e schema.StreamError
			if err := evt.Json(&e); err != nil {
				return fmt.Errorf("malformed error event: %w", err)
			}
			streamErr = fmt.Errorf("%s", e.Error)
		case schema.EventResult:
			var m schema.Model
			if err := evt.Json(&m); err != nil {
				return fmt.Errorf("malformed result event: %w", err)
			}
			response = &m
		}
		return nil
	}

	var discard struct{}
	if err := c.DoWithContext(ctx, payload, &discard,
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
