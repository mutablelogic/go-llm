package gowhisper

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Downloader = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// LoadModel downloads the model with the name, which is its file, such as
// "ggml-tiny.en.bin", from the whisper.cpp models on Hugging Face, or its
// id, such as "ggml-tiny.en", and returns it, or returns it if it's already
// downloaded. Returns ErrNotFound when there's no such model to download. The progress function, if not nil, is called as it downloads,
// with the percentage of its bytes downloaded. The download isn't timed out,
// as models can be large, but is cancelled with the context.
func (c *Client) LoadModel(ctx context.Context, name string, progress llm.DownloadProgressFn) (llm.Model, error) {
	file := modelFile(name)
	if file == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	}

	// A model which is downloaded is returned
	if model, err := c.GetModel(ctx, file); err == nil {
		return model, nil
	} else if !isNotFound(err) {
		return nil, err
	}

	// Download the model, streaming its progress when there's a function to
	// call with it, or otherwise waiting for it
	req, err := client.NewJSONRequest(downloadRequest{Model: file})
	if err != nil {
		return nil, err
	}
	var data apiModel
	opts := []client.RequestOpt{client.OptPath("model"), client.OptNoTimeout()}
	if progress != nil {
		var done bool
		opts = append(opts, client.OptReqHeader("Accept", types.ContentTypeTextStream), client.OptTextStreamCallback(func(event client.TextStreamEvent) error {
			switch event.Event {
			case eventProgress:
				var status downloadProgress
				if err := event.Json(&status); err != nil {
					return err
				}
				var percent float64
				if status.Total > 0 {
					percent = float64(status.Current) / float64(status.Total) * 100
				}
				progress("downloading", percent)
			case eventError:
				return downloadErr(file, event.Data)
			case eventDone:
				done = true
				return event.Json(&data)
			}
			return nil
		}))
		if err := c.DoWithContext(ctx, req, nil, opts...); err != nil {
			return nil, notFound(file, err)
		} else if !done {
			return nil, llm.ErrServiceUnavailable.With("the download ended before it was done")
		}
	} else if err := c.DoWithContext(ctx, req, &data, opts...); err != nil {
		return nil, notFound(file, err)
	}
	return &Model{provider: c, data: data}, nil
}

// DeleteModel deletes the model, which must be a go-whisper model, from
// the server
func (c *Client) DeleteModel(ctx context.Context, model llm.Model) error {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.GoWhisper {
		return llm.ErrBadParameter.With("not a gowhisper model")
	}
	req := client.NewRequestEx(http.MethodDelete, types.ContentTypeAny)
	return c.DoWithContext(ctx, req, nil, client.OptPath("model", modelID(model.Name())))
}

// UnloadModel returns ErrNotImplemented, as go-whisper loads a model each
// time it's used
func (c *Client) UnloadModel(ctx context.Context, model llm.Model) error {
	return llm.ErrNotImplemented.With("gowhisper doesn't keep models loaded")
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// downloadErr returns the error of a download's error event, whose data is
// its message, as a JSON string, which is ErrNotFound when the model's file
// isn't found, as go-whisper's message has the status of the request for it
func downloadErr(file, data string) error {
	var message string
	if err := json.Unmarshal([]byte(data), &message); err != nil {
		message = data
	}
	message = strings.TrimSpace(message)
	if strings.HasPrefix(message, http.StatusText(http.StatusNotFound)) {
		return llm.ErrNotFound.Withf("model %q isn't found to download", file)
	}
	return errors.New(message)
}

// notFound returns the error of a download, which is ErrNotFound when the
// model's file isn't found, without go-whisper's response, which only says
// it isn't found
func notFound(file string, err error) error {
	if isNotFound(err) {
		return llm.ErrNotFound.Withf("model %q isn't found to download", file)
	}
	return err
}

// isNotFound returns true when the error is a model which isn't found
func isNotFound(err error) bool {
	if errors.Is(err, llm.ErrNotFound) {
		return true
	}
	status, ok := llm.HTTPStatus(err)
	return ok && status == http.StatusNotFound
}
