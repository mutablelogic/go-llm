package ollama

import (
	"context"
	"slices"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListVoices returns the voices the model speaks in, by their names, which
// Ollama shows for a speech model, such as phonon, or none, when it shows
// none. Returns ErrNotImplemented for a model which doesn't speak.
func (c *Client) ListVoices(ctx context.Context, model llm.Model, opts ...llm.Opt) ([]llm.Voice, string, error) {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.Ollama {
		return nil, "", llm.ErrBadParameter.With("not an ollama model")
	}

	// The model, as it's shown, as one which is listed may not have its
	// voices
	data, err := c.show(ctx, model.Name())
	if err != nil {
		return nil, "", err
	} else if len(data.Capabilities) > 0 && !slices.Contains(data.Capabilities, "speech") {
		return nil, "", llm.ErrNotImplemented.Withf("%s doesn't speak", model.Name())
	}
	result := make([]llm.Voice, 0, len(data.Voices))
	for _, name := range data.Voices {
		if name != "" {
			result = append(result, llm.Voice{ID: name, Title: name})
		}
	}
	return llm.Paged(result, opts...)
}
