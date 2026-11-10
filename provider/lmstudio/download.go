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

// downloadPoll is how often a download's status is asked for, and, once
// it's downloaded, whether it's listed, for at most listWait, as LM Studio
// lists a model a while after its download completes
var (
	downloadPoll = time.Second
	listWait     = 30 * time.Second
)

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

	// Download it, unless it's stored, and then find it, by its key, which
	// a Hugging Face link isn't
	models, err := c.listModels(ctx)
	if err != nil {
		return nil, err
	}
	var model llm.Model
	if data := findModel(models, name, nil); data != nil {
		model = &Model{provider: c, data: *data}
	} else {
		before := make(map[string]bool, len(models))
		for _, data := range models {
			before[data.Key] = true
		}
		if err := c.download(ctx, name, progress); err != nil {
			return nil, err
		}
		if model, err = c.waitListed(ctx, name, before); errors.Is(err, llm.ErrNotFound) {
			return nil, llm.ErrNotFound.Withf("model %q was downloaded, but isn't listed by that name: load it by its name in the list of models", name)
		} else if err != nil {
			return nil, err
		}
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

// waitListed returns the model which was downloaded by the name, once it's
// listed, which LM Studio does a while after it's downloaded, as findModel
// finds it, or ErrNotFound, when it isn't listed within listWait
func (c *Client) waitListed(ctx context.Context, name string, before map[string]bool) (llm.Model, error) {
	deadline := time.Now().Add(listWait)
	for {
		models, err := c.listModels(ctx)
		if err != nil {
			return nil, err
		} else if data := findModel(models, name, before); data != nil {
			return &Model{provider: c, data: *data}, nil
		} else if time.Now().After(deadline) {
			return nil, llm.ErrNotFound.Withf("model %q not found", name)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(downloadPoll):
		}
	}
}

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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// findModel returns the model with the name, as its key, or, with the keys
// of the models before one was downloaded, the one model which is new, and
// whose key is for the same model as the name, such as qwen3-4b, for
// qwen/qwen3-4b, or for a link to a Hugging Face repository, Qwen3-4B-GGUF,
// or, for a name which is a link, the one model of the repository, which
// was downloaded already, or nil. A model which is new, but for another
// model, such as one another download added, isn't the one, so it's waited
// for.
func findModel(models []apiModel, name string, before map[string]bool) *apiModel {
	for i := range models {
		if models[i].Key == name {
			return &models[i]
		}
	}
	repo := repoName(name)
	if before == nil || repo == "" {
		return nil
	}
	match := func(new bool) *apiModel {
		var found []int
		for i := range models {
			if (!new || !before[models[i].Key]) && repoName(models[i].Key) == repo {
				found = append(found, i)
			}
		}
		if len(found) == 1 {
			return &models[found[0]]
		}
		return nil
	}
	if model := match(true); model != nil {
		return model
	} else if strings.Contains(name, "://") {
		return match(false)
	}
	return nil
}

// repoName returns the name of the model a key, or a Hugging Face link,
// is for, in lower case, which is its last part, without a format, such as
// -GGUF, which LM Studio's keys haven't, so "qwen3-4b" for
// "https://huggingface.co/lmstudio-community/Qwen3-4B-GGUF", and for
// "qwen/qwen3-4b"
func repoName(name string) string {
	name = strings.ToLower(strings.TrimRight(strings.TrimSpace(name), "/"))
	if i := strings.Index(name, "?"); i >= 0 {
		name = name[:i]
	}
	name = name[strings.LastIndex(name, "/")+1:]
	for _, format := range []string{"-gguf", "_gguf", "-mlx", "_mlx"} {
		name = strings.TrimSuffix(name, format)
	}
	return name
}
