package llm_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	llm "github.com/mutablelogic/go-llm/toolkit/llm"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// models is a fake of the registry, whose model speaks, as audio, the text
// it's given, and keeps the options it was given
type models struct {
	opts *gollm.Opts
}

type model struct{ name string }

func (m model) Name() string                   { return m.name }
func (model) Provider() gollm.Provider         { return nil }
func (model) Description() string              { return "" }
func (model) Created() time.Time               { return time.Time{} }
func (model) Aliases() []string                { return nil }
func (model) Meta() map[string]json.RawMessage { return nil }
func (model) InputTokenLimit() *uint           { return nil }
func (model) OutputTokenLimit() *uint          { return nil }
func (model) Capabilities() gollm.ModelCap     { return gollm.ModelCap{} }

func (m *models) GetModel(_ context.Context, name string) (gollm.Model, error) {
	if name != "phonon" {
		return nil, gollm.ErrNotFound.Withf("model %q", name)
	}
	return model{name: name}, nil
}

func (m *models) Speak(_ context.Context, _ gollm.Model, text string, opts ...gollm.Opt) (gollm.Resource, gollm.Usage, error) {
	o, err := gollm.ApplyOpts(opts...)
	if err != nil {
		return nil, gollm.Usage{}, err
	}
	m.opts = o
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wav", Data: []byte(text)}}, gollm.Usage{}, nil
}

func Test_Speak(t *testing.T) {
	ctx := context.Background()
	m := &models{}

	// Without a model to speak with, there's no speak tool
	ns, err := llm.New(func() llm.Models { return m })
	require.NoError(t, err)
	assert.Equal(t, "llm", ns.Name())
	assert.Empty(t, ns.Tools())
	_, err = llm.New(nil)
	assert.ErrorIs(t, err, gollm.ErrBadParameter)

	// With one, the tool speaks text, in its voice, or the one given, with
	// the instructions, and returns the audio, named speech, with its
	// format's extension
	ns, err = llm.New(func() llm.Models { return m }, llm.WithSpeak("phonon", "Freya"))
	require.NoError(t, err)
	require.Len(t, ns.Tools(), 1)
	tool := ns.Tools()[0]
	assert.Equal(t, "speak", tool.Name())
	result, err := tool.Run(ctx, json.RawMessage(`{"text":"Hello"}`))
	require.NoError(t, err)
	audio := result.(gollm.Resource)
	assert.Equal(t, "speech.wav", audio.Name())
	assert.Equal(t, "audio/wav", audio.Type())
	voice, _ := gollm.GetOpt[string](m.opts, gollm.VoiceKey)
	assert.Equal(t, "Freya", voice)
	_, err = tool.Run(ctx, json.RawMessage(`{"text":"Hello","voice":"Hugh","instructions":"Calmly"}`))
	require.NoError(t, err)
	voice, _ = gollm.GetOpt[string](m.opts, gollm.VoiceKey)
	instructions, _ := gollm.GetOpt[string](m.opts, gollm.InstructionsKey)
	assert.Equal(t, "Hugh", voice)
	assert.Equal(t, "Calmly", instructions)

	// In a toolkit, with a store, its result is a link to the audio
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(ns))
	stored := toolkit.WithStore(ctx, func(ctx context.Context, r gollm.Resource) (schema.ResourceLink, error) {
		return schema.ResourceLink{URI: "resource://abc", Name: r.Name(), Type: r.Type()}, nil
	})
	link, err := tk.CallTool(stored, "llm:speak", json.RawMessage(`{"text":"Hello"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"resources":[{"uri":"resource://abc","name":"speech.wav","type":"audio/wav"}]}`, string(link))

	// No text, and no models yet, aren't spoken
	_, err = tool.Run(ctx, json.RawMessage(`{"text":" "}`))
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	none, err := llm.New(func() llm.Models { return nil }, llm.WithSpeak("phonon", ""))
	require.NoError(t, err)
	_, err = none.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello"}`))
	assert.ErrorIs(t, err, gollm.ErrServiceUnavailable)
}
