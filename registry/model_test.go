package registry_test

import (
	"context"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// FAKES

// capable is a provider which generates, downloads, embeds, transcribes and
// recognizes, and records what it was called for, and the model
type capable struct {
	*provider
	called []string
}

var _ llm.Generator = (*capable)(nil)
var _ llm.Downloader = (*capable)(nil)
var _ llm.Embedder = (*capable)(nil)
var _ llm.Transcriber = (*capable)(nil)
var _ llm.Recognizer = (*capable)(nil)

func newCapable(name string, models ...string) *capable {
	c := &capable{provider: &provider{name: name, models: models}}
	return c
}

func (c *capable) Self() llm.Provider { return c }

func (c *capable) GetModel(ctx context.Context, name string) (llm.Model, error) {
	if _, err := c.provider.GetModel(ctx, name); err != nil {
		return nil, err
	}
	return &model{name: name, provider: c}, nil
}

func (c *capable) ListModels(ctx context.Context) ([]llm.Model, error) {
	c.lists.Add(1)
	var result []llm.Model
	for _, name := range c.models {
		result = append(result, &model{name: name, provider: c})
	}
	return result, nil
}

func (c *capable) record(action string, model llm.Model) {
	c.called = append(c.called, action+" "+model.Name())
}

func (c *capable) WithoutSession(_ context.Context, model llm.Model, _ []llm.Message, _ []llm.Tool, _ llm.GeneratorStreamFn, _ ...llm.Opt) (llm.Message, llm.Usage, error) {
	c.record("generate", model)
	message, err := schema.NewMessage("assistant", "hello")
	return message, llm.Usage{}, err
}

func (c *capable) Embed(_ context.Context, model llm.Model, messages []llm.Message, _ ...llm.Opt) ([]llm.Vector, llm.Usage, error) {
	c.record("embed", model)
	return make([]llm.Vector, len(messages)), llm.Usage{}, nil
}

func (c *capable) Transcribe(_ context.Context, model llm.Model, _ llm.Resource, _ ...llm.Opt) (*llm.Transcript, llm.Usage, error) {
	c.record("transcribe", model)
	return &llm.Transcript{Text: "hello"}, llm.Usage{}, nil
}

// Translate translates, except with a model named transcribe-only, which
// can only transcribe
func (c *capable) Translate(_ context.Context, model llm.Model, _ llm.Resource, _ ...llm.Opt) (*llm.Transcript, llm.Usage, error) {
	if model.Name() == "transcribe-only" {
		return nil, llm.Usage{}, llm.ErrNotImplemented.Withf("model %q can't translate", model.Name())
	}
	c.record("translate", model)
	return &llm.Transcript{Text: "hello", Language: "en"}, llm.Usage{}, nil
}

func (c *capable) Recognize(_ context.Context, model llm.Model, _ llm.Resource, _ ...llm.Opt) (llm.Message, llm.Usage, error) {
	c.record("recognize", model)
	message, err := schema.NewMessage("assistant", "# Page 1")
	return message, llm.Usage{}, err
}

func (c *capable) LoadModel(_ context.Context, name string, _ llm.DownloadProgressFn) (llm.Model, error) {
	m := &model{name: name, provider: c}
	c.record("download", m)
	return m, nil
}

func (c *capable) DeleteModel(_ context.Context, model llm.Model) error {
	c.record("delete", model)
	return nil
}

func (c *capable) UnloadModel(_ context.Context, model llm.Model) error {
	c.record("unload", model)
	return nil
}

// user returns a message from the user with the text
func user(t *testing.T, text string) llm.Message {
	t.Helper()
	message, err := schema.NewMessage("user", text)
	require.NoError(t, err)
	return message
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_Capabilities(t *testing.T) {
	ctx := context.Background()
	ollama := newCapable("ollama", "llama3.2:latest", "transcribe-only")
	openai := &provider{name: "openai", models: []string{"gpt-4o"}}
	client, err := eliza.New(eliza.WithSeed(42))
	require.NoError(t, err)
	r, err := newRegistry(ollama, openai, client)
	require.NoError(t, err)

	// get returns the model with the name from the registry
	get := func(t *testing.T, name string) llm.Model {
		t.Helper()
		model, err := r.GetModel(ctx, name)
		require.NoError(t, err)
		return model
	}

	t.Run("Generate", func(t *testing.T) {
		// The model's provider generates the reply
		ollama.called = nil
		reply, _, err := r.WithoutSession(ctx, get(t, "llama3.2:latest"), []llm.Message{user(t, "Hi")}, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "hello", reply.Text())
		assert.Equal(t, []string{"generate llama3.2:latest"}, ollama.called)

		// Including eliza, which really replies
		reply, _, err = r.WithoutSession(ctx, get(t, eliza.ModelName), []llm.Message{user(t, "I feel sad")}, nil, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, reply.Text())
	})

	t.Run("Embed", func(t *testing.T) {
		ollama.called = nil
		vectors, _, err := r.Embed(ctx, get(t, "llama3.2:latest"), []llm.Message{user(t, "a"), user(t, "b")})
		require.NoError(t, err)
		assert.Len(t, vectors, 2)
		assert.Equal(t, []string{"embed llama3.2:latest"}, ollama.called)
	})

	t.Run("Transcribe", func(t *testing.T) {
		ollama.called = nil
		transcript, _, err := r.Transcribe(ctx, get(t, "llama3.2:latest"), nil)
		require.NoError(t, err)
		assert.Equal(t, "hello", transcript.Text)
		assert.Equal(t, []string{"transcribe llama3.2:latest"}, ollama.called)
	})

	t.Run("Translate", func(t *testing.T) {
		// The model's provider translates
		ollama.called = nil
		transcript, _, err := r.Translate(ctx, get(t, "llama3.2:latest"), nil)
		require.NoError(t, err)
		assert.Equal(t, "en", transcript.Language)
		assert.Equal(t, []string{"translate llama3.2:latest"}, ollama.called)

		// Unless the model can only transcribe
		_, _, err = r.Translate(ctx, get(t, "transcribe-only"), nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		_, _, err = r.Transcribe(ctx, get(t, "transcribe-only"), nil)
		assert.NoError(t, err)

		// Or the provider can't transcribe at all
		_, _, err = r.Translate(ctx, get(t, "gpt-4o"), nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		assert.ErrorContains(t, err, `provider "openai" can't translate audio`)
	})

	t.Run("Recognize", func(t *testing.T) {
		// The model's provider recognizes the text
		ollama.called = nil
		page := &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "image/png", Data: []byte("png")}}
		message, _, err := r.Recognize(ctx, get(t, "llama3.2:latest"), page)
		require.NoError(t, err)
		assert.Equal(t, "# Page 1", message.Text())
		assert.Equal(t, []string{"recognize llama3.2:latest"}, ollama.called)

		// Unless it can't
		_, _, err = r.Recognize(ctx, get(t, eliza.ModelName), page)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		assert.ErrorContains(t, err, `provider "eliza" can't recognize text`)
	})

	t.Run("NotImplemented", func(t *testing.T) {
		// A provider which can't do it, such as eliza embedding, or a
		// provider which can do nothing but list models
		_, _, err := r.Embed(ctx, get(t, eliza.ModelName), []llm.Message{user(t, "a")})
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		assert.ErrorContains(t, err, `provider "eliza" can't embed messages`)
		_, _, err = r.WithoutSession(ctx, get(t, "gpt-4o"), []llm.Message{user(t, "Hi")}, nil, nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		_, _, err = r.Transcribe(ctx, get(t, "gpt-4o"), nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		assert.ErrorIs(t, r.DeleteModel(ctx, get(t, "gpt-4o")), llm.ErrNotImplemented)
		assert.ErrorIs(t, r.UnloadModel(ctx, get(t, "gpt-4o")), llm.ErrNotImplemented)
	})

	t.Run("NotInRegistry", func(t *testing.T) {
		// A model whose provider isn't in the registry, or was removed
		other := newCapable("other", "model")
		model, err := other.GetModel(ctx, "model")
		require.NoError(t, err)
		_, _, err = r.WithoutSession(ctx, model, []llm.Message{user(t, "Hi")}, nil, nil)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		assert.Empty(t, other.called)

		_, _, err = r.Embed(ctx, nil, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Download", func(t *testing.T) {
		// A canonical name is downloaded by its provider
		ollama.called = nil
		model, err := r.LoadModel(ctx, "ollama:phi4:latest", nil)
		require.NoError(t, err)
		assert.Equal(t, "ollama:phi4:latest", r.Name(model))
		assert.Equal(t, []string{"download phi4:latest"}, ollama.called)

		// And so is a name, as ollama is the only provider which downloads
		model, err = r.LoadModel(ctx, "phi4:latest", nil)
		require.NoError(t, err)
		assert.Equal(t, "ollama:phi4:latest", r.Name(model))

		// And a model is deleted by its provider
		ollama.called = nil
		require.NoError(t, r.DeleteModel(ctx, model))
		assert.Equal(t, []string{"delete phi4:latest"}, ollama.called)

		// And unloaded by its provider, which keeps it, so its models aren't
		// listed again
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		ollama.called = nil
		lists := ollama.lists.Load()
		require.NoError(t, r.UnloadModel(ctx, model))
		assert.Equal(t, []string{"unload phi4:latest"}, ollama.called)
		_, err = r.ListModels(ctx)
		require.NoError(t, err)
		assert.Equal(t, lists, ollama.lists.Load())

		// But not by a provider which can't download
		_, err = r.LoadModel(ctx, "openai:gpt-4o", nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		_, err = r.LoadModel(ctx, "", nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("DownloadAmbiguous", func(t *testing.T) {
		// With more than one provider which downloads, the name needs its
		// provider
		second := newCapable("lmstudio")
		require.NoError(t, r.Add("", second))
		defer r.Remove("lmstudio")
		_, err := r.LoadModel(ctx, "phi4:latest", nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.True(t, strings.Contains(err.Error(), "lmstudio and ollama"), err.Error())
		assert.Empty(t, second.called)

		// But not with its canonical name
		_, err = r.LoadModel(ctx, "lmstudio:phi4:latest", nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"download phi4:latest"}, second.called)
	})

	t.Run("NoDownloader", func(t *testing.T) {
		r, err := newRegistry(openai)
		require.NoError(t, err)
		_, err = r.LoadModel(ctx, "phi4:latest", nil)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
	})
}
