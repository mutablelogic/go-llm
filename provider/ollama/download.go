package ollama

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
// TYPES

var _ llm.Downloader = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultTag is the tag of a model name without one
const defaultTag = "latest"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// LoadModel downloads the model with the name, and returns it, or returns
// it if it's already present. A name without a tag has the default tag,
// "latest", as Ollama stores and lists the model with it, so the model is
// returned with its full name, such as "gemma:latest" for "gemma". The
// progress function, if not nil, is called with the status of the download,
// and the percentage of its bytes downloaded, as it progresses. The
// download isn't timed out, as models can be large, but is cancelled with
// the context.
func (c *Client) LoadModel(ctx context.Context, name string, progress llm.DownloadProgressFn) (llm.Model, error) {
	name = modelName(name)
	req, err := client.NewJSONRequest(map[string]any{
		"model":  name,
		"stream": progress != nil,
	})
	if err != nil {
		return nil, err
	}

	// Pull the model, calling the progress function with each status
	var status pullStatus
	opts := []client.RequestOpt{client.OptPath("pull"), client.OptNoTimeout()}
	if progress != nil {
		opts = append(opts, client.OptJsonStreamCallback(func(data json.RawMessage) error {
			var status pullStatus
			if err := json.Unmarshal(data, &status); err != nil {
				return err
			} else if status.Error != "" {
				return errors.New(status.Error)
			}
			var percent float64
			if status.Total > 0 {
				percent = float64(status.Completed) / float64(status.Total) * 100
			}
			progress(status.Status, percent)
			return nil
		}))
	}
	if err := c.DoWithContext(ctx, req, &status, opts...); err != nil {
		return nil, err
	} else if status.Error != "" {
		return nil, errors.New(status.Error)
	}

	// Return the downloaded model
	return c.GetModel(ctx, name)
}

// DeleteModel deletes the model, which must be an Ollama model, from the
// server
func (c *Client) DeleteModel(ctx context.Context, model llm.Model) error {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.Ollama {
		return llm.ErrBadParameter.With("not an ollama model")
	}
	req, err := client.NewJSONRequestEx(http.MethodDelete, map[string]string{
		"model": model.Name(),
	}, types.ContentTypeAny)
	if err != nil {
		return err
	}
	return c.DoWithContext(ctx, req, nil, client.OptPath("delete"))
}

// UnloadModel unloads the model, which must be an Ollama model, from the
// server's memory, keeping it stored, with a request to generate with it
// which keeps it loaded for no time, as Ollama does
func (c *Client) UnloadModel(ctx context.Context, model llm.Model) error {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.Ollama {
		return llm.ErrBadParameter.With("not an ollama model")
	}
	req, err := client.NewJSONRequest(map[string]any{
		"model":      model.Name(),
		"keep_alive": 0,
		"stream":     false,
	})
	if err != nil {
		return err
	}
	return c.DoWithContext(ctx, req, nil, client.OptPath("generate"))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// modelName returns the full form of an Ollama model name, which is
// [registry/][namespace/]model[:tag], with the default tag, "latest", if it
// has none, as Ollama stores and lists models with their tag. A registry can
// have a port, such as "registry.example:5000/team/model", so the tag is
// after the last "/".
func modelName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	model := name[strings.LastIndex(name, "/")+1:]
	if strings.Contains(model, ":") {
		return name
	}
	return name + ":" + defaultTag
}
