package httpclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns the models of the providers, or of the provider which
// the request selects, in order of their providers' names, and then their
// names
func (c *Client) ListModels(ctx context.Context, req schema.ModelListRequest) (*schema.ModelList, error) {
	var response schema.ModelList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "model"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

// GetModel returns the model with the name, or alias, or provider and name,
// separated by a colon, such as my-ollama:llama3.2:latest. The name is one
// segment of the path, so a "/" in it is escaped.
func (c *Client) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	var response schema.Model
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "model", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

// LoadModel loads the model in the request, with its provider, or the one
// provider which loads models, which downloads it when it isn't stored, and
// returns it. Loading isn't timed out, as models can be large, but is
// cancelled with the context. When the progress function isn't nil, the
// download's progress is streamed, and the function is called with each
// progress event.
func (c *Client) LoadModel(ctx context.Context, req schema.LoadModelRequest, progress llm.DownloadProgressFn) (*schema.Model, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Name == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	} else if progress != nil {
		return c.loadModelStream(ctx, req, progress)
	}
	body, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}
	var response schema.Model
	if err := c.DoWithContext(ctx, body, &response, client.OptPath("llm", "model"), client.OptNoTimeout()); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteModel deletes the model with the name, or alias, or provider and
// name, such as my-ollama:gemma:latest, from its provider's storage, and
// returns it
func (c *Client) DeleteModel(ctx context.Context, name string) (*schema.Model, error) {
	if strings.TrimSpace(name) == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}
	var response schema.Model
	if err := c.DoWithContext(ctx, client.MethodDelete, &response, client.OptPath("llm", "model", name)); err != nil {
		return nil, err
	}
	return &response, nil
}

// UnloadModel unloads the model with the name, or alias, or provider and
// name, such as my-ollama:gemma:latest, from its provider's memory, keeping
// it stored, and returns it. The name is one segment of the path, so a "/"
// in it is escaped.
func (c *Client) UnloadModel(ctx context.Context, name string) (*schema.Model, error) {
	if strings.TrimSpace(name) == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}
	var response schema.Model
	if err := c.DoWithContext(ctx, client.MethodPost, &response, client.OptPath("llm", "model", name, "unload")); err != nil {
		return nil, err
	}
	return &response, nil
}

// ListVoices returns the voices of the model with the name, or alias, or
// provider and name, such as elevenlabs:eleven_v4_turbo, which the request
// selects, with the token of the next page, when there are more. The name is
// one segment of the path, so a "/" in it is escaped.
func (c *Client) ListVoices(ctx context.Context, name string, req schema.VoiceListRequest) (*schema.VoiceList, error) {
	if strings.TrimSpace(name) == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}
	var response schema.VoiceList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "model", name, "voice"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// loadModelStream loads a model, accepting server-sent events, and calls
// the progress function with each progress event. Returns the model of the
// result event, or the error of an error event.
func (c *Client) loadModelStream(ctx context.Context, req schema.LoadModelRequest, progress llm.DownloadProgressFn) (*schema.Model, error) {
	body, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	// Read the events
	var result *schema.Model
	var resultErr error
	callback := func(event client.TextStreamEvent) error {
		switch event.Event {
		case schema.EventProgress:
			var data schema.ProgressEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed progress event: %w", err)
			}
			progress(data.Status, data.Percent)
		case schema.EventError:
			var data schema.ErrorEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed error event: %w", err)
			}
			resultErr = errors.New(data.Error)
		case schema.EventResult:
			var data schema.Model
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed result event: %w", err)
			}
			result = &data
		}
		return nil
	}
	if err := c.DoWithContext(ctx, body, nil,
		client.OptPath("llm", "model"),
		client.OptReqHeader("Accept", types.ContentTypeTextStream),
		client.OptTextStreamCallback(callback),
		client.OptNoTimeout(),
	); err != nil {
		return nil, err
	}

	// Return the model, or the error
	switch {
	case resultErr != nil:
		return nil, resultErr
	case result == nil:
		return nil, errors.New("the stream ended without a result")
	default:
		return result, nil
	}
}
