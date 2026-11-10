package lmstudio

import (
	"context"
	"errors"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

var _ llm.Downloader = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// downloadPoll is how often a download's status is asked for
var downloadPoll = time.Second

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// LoadModel downloads the model with the name, when it isn't stored, which
// is a model of LM Studio's catalog, such as "google/gemma-4-12b-qat", or a
// Hugging Face link, and loads it, in memory, unless it's loaded already,
// with LM Studio's settings, such as its context length, and returns it.
// The progress function, if not nil, is called with the status of the
// download, and the percentage of its bytes downloaded, as it progresses,
// and as the model is loaded. Neither is timed out, as models can be large,
// but both are cancelled with the context.
func (c *Client) LoadModel(ctx context.Context, name string, progress llm.DownloadProgressFn) (llm.Model, error) {
	if progress == nil {
		progress = func(string, float64) {}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, llm.ErrBadParameter.With("missing model name")
	}

	// Download it, unless it's stored
	model, err := c.GetModel(ctx, name)
	if errors.Is(err, llm.ErrNotFound) {
		if err := c.download(ctx, name, progress); err != nil {
			return nil, err
		}
		if model, err = c.GetModel(ctx, name); errors.Is(err, llm.ErrNotFound) {
			return nil, llm.ErrNotFound.Withf("model %q was downloaded, but isn't listed by that name", name)
		}
	}
	if err != nil {
		return nil, err
	}

	// Load it, unless it's loaded
	if model.(*Model).Loaded() {
		return model, nil
	}
	progress("loading", 0)
	req, err := client.NewJSONRequest(map[string]any{"model": model.Name()})
	if err != nil {
		return nil, err
	}
	var loaded loadResponse
	if err := c.DoWithContext(ctx, req, &loaded, client.OptPath("models", "load"), client.OptNoTimeout()); err != nil {
		return nil, err
	}
	progress("loaded", 100)
	return c.GetModel(ctx, model.Name())
}

// UnloadModel unloads the model, which must be an LM Studio model, from the
// server's memory, keeping it stored, which is every instance of it which is
// loaded, so it's loaded again when it's next used, when LM Studio is set to
// load models as they're needed
func (c *Client) UnloadModel(ctx context.Context, model llm.Model) error {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.LMStudio {
		return llm.ErrBadParameter.With("not an LM Studio model")
	}

	// The instances which are loaded now
	current, err := c.GetModel(ctx, model.Name())
	if err != nil {
		return err
	}
	for _, instance := range current.(*Model).data.LoadedInstances {
		req, err := client.NewJSONRequest(map[string]any{"instance_id": instance.ID})
		if err != nil {
			return err
		}
		if err := c.DoWithContext(ctx, req, nil, client.OptPath("models", "unload")); err != nil {
			return err
		}
	}
	return nil
}

// DeleteModel returns ErrNotImplemented, as LM Studio's API doesn't delete
// models, which are deleted in LM Studio
func (c *Client) DeleteModel(context.Context, llm.Model) error {
	return llm.ErrNotImplemented.With("LM Studio's API doesn't delete models, which are deleted in LM Studio")
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// download downloads the model with the name, and waits for it to be
// downloaded, calling the progress function with how it's getting on
func (c *Client) download(ctx context.Context, name string, progress llm.DownloadProgressFn) error {
	req, err := client.NewJSONRequest(map[string]any{"model": name})
	if err != nil {
		return err
	}
	var status downloadStatus
	if err := c.DoWithContext(ctx, req, &status, client.OptPath("models", "download")); err != nil {
		return err
	}
	ticker := time.NewTicker(downloadPoll)
	defer ticker.Stop()
	for {
		switch status.Status {
		case downloadDone, downloadCompleted:
			progress(downloadCompleted, 100)
			return nil
		case downloadFailed:
			return llm.ErrInternalServerError.Withf("model %q couldn't be downloaded", name)
		}
		var percent float64
		if status.Total > 0 {
			percent = min(100, float64(status.Downloaded)/float64(status.Total)*100)
		}
		progress(status.Status, percent)
		if status.JobID == "" {
			return llm.ErrInternalServerError.Withf("model %q is %s, without a download to follow", name, status.Status)
		}

		// Wait, and ask how it's getting on
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		job := status.JobID
		status = downloadStatus{}
		if err := c.DoWithContext(ctx, nil, &status, client.OptPath("models", "download", "status", job)); err != nil {
			return err
		}
		if status.JobID == "" {
			status.JobID = job
		}
	}
}
