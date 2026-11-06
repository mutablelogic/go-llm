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

func (m *models) Recognize(_ context.Context, _ gollm.Model, resource gollm.Resource, opts ...gollm.Opt) (gollm.Document, gollm.Usage, error) {
	o, err := gollm.ApplyOpts(opts...)
	if err != nil {
		return gollm.Document{}, gollm.Usage{}, err
	}
	m.opts = o
	data, err := gollm.ReadAll(context.Background(), resource)
	if err != nil {
		return gollm.Document{}, gollm.Usage{}, err
	}
	return gollm.Document{Pages: []gollm.DocumentPage{{Text: "# " + string(data)}, {Text: "Page two"}}}, gollm.Usage{Pages: 2}, nil
}

func (m *models) Transcribe(_ context.Context, _ gollm.Model, _ gollm.Resource, _ gollm.TranscriberStreamFn, opts ...gollm.Opt) (gollm.Transcript, gollm.Usage, error) {
	o, err := gollm.ApplyOpts(opts...)
	if err != nil {
		return gollm.Transcript{}, gollm.Usage{}, err
	}
	m.opts = o
	return gollm.Transcript{Text: "Bonjour", Language: "fr"}, gollm.Usage{}, nil
}

func (m *models) Translate(context.Context, gollm.Model, gollm.Resource, gollm.TranscriberStreamFn, ...gollm.Opt) (gollm.Transcript, gollm.Usage, error) {
	return gollm.Transcript{Text: "Hello", Language: "en"}, gollm.Usage{}, nil
}

func Test_Speak(t *testing.T) {
	ctx := context.Background()
	m := &models{}

	// Without a default model, the tools are there, and need a model, which
	// the model which calls them gives
	ns, err := llm.New(func() llm.Models { return m })
	require.NoError(t, err)
	assert.Equal(t, "llm", ns.Name())
	require.Len(t, ns.Tools(), 3)
	assert.Contains(t, ns.Tools()[0].Description(), "there's no default")
	_, err = ns.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello"}`))
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	given, err := ns.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello","model":"phonon"}`))
	require.NoError(t, err)
	assert.Equal(t, "speech.wav", given.(gollm.Resource).Name())
	_, err = ns.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello","model":"missing"}`))
	assert.ErrorIs(t, err, gollm.ErrNotFound)
	_, err = llm.New(nil)
	assert.ErrorIs(t, err, gollm.ErrBadParameter)

	// With one, the tool speaks text, in its voice, or the one given, with
	// the instructions, and returns the audio, named speech, with its
	// format's extension
	ns, err = llm.New(func() llm.Models { return m }, llm.WithSpeak("phonon", "Freya"))
	require.NoError(t, err)
	tool := ns.Tools()[0]
	assert.Contains(t, tool.Description(), "It uses phonon")
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

	// The ocr, and transcribe, tools take the model which is given, too
	ns, err = llm.New(func() llm.Models { return m })
	require.NoError(t, err)
	tk2, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk2.Add(ns))
	resolved := toolkit.WithResolver(ctx, func(context.Context, string) (gollm.Resource, error) {
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wav", Data: []byte("x")}}, nil
	})
	_, err = tk2.CallTool(resolved, "llm:transcribe", json.RawMessage(`{"uri":"resource://abc"}`))
	assert.ErrorIs(t, err, gollm.ErrBadParameter, "no model, and no default")
	_, err = tk2.CallTool(resolved, "llm:transcribe", json.RawMessage(`{"uri":"resource://abc","model":"phonon"}`))
	assert.NoError(t, err)
}

func Test_ReadTools(t *testing.T) {
	ctx := context.Background()
	m := &models{}
	ns, err := llm.New(func() llm.Models { return m }, llm.WithOCR("phonon"), llm.WithTranscribe("phonon"))
	require.NoError(t, err)
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(ns))

	// The tools read resources by their URIs, with the context's resolver,
	// which resolves only the one it has
	resolved := toolkit.WithResolver(ctx, func(_ context.Context, uri string) (gollm.Resource, error) {
		if uri != "resource://abc" {
			return nil, gollm.ErrNotFound.Withf("resource %s", uri)
		}
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "application/pdf", Data: []byte("Report")}}, nil
	})

	// The text of a document, as Markdown, with its pages, which are asked
	// for
	result, err := tk.CallTool(resolved, "llm:ocr", json.RawMessage(`{"uri":"resource://abc","pages":"0-1"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"text":"# Report\n\n---\n\nPage two","pages":2}`, string(result))
	pages, _ := gollm.GetOpt[string](m.opts, gollm.PagesKey)
	assert.Equal(t, "0-1", pages)

	// Speech, transcribed, in its language, which is given, or translated
	result, err = tk.CallTool(resolved, "llm:transcribe", json.RawMessage(`{"uri":"resource://abc","language":"fr"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"text":"Bonjour","language":"fr"}`, string(result))
	language, _ := gollm.GetOpt[string](m.opts, gollm.LanguageKey)
	assert.Equal(t, "fr", language)
	result, err = tk.CallTool(resolved, "llm:transcribe", json.RawMessage(`{"uri":"resource://abc","translate":true}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"text":"Hello","language":"en"}`, string(result))

	// A resource which isn't found, or without a resolver, isn't read
	_, err = tk.CallTool(resolved, "llm:ocr", json.RawMessage(`{"uri":"resource://xyz"}`))
	assert.ErrorIs(t, err, gollm.ErrNotFound)
	_, err = tk.CallTool(ctx, "llm:ocr", json.RawMessage(`{"uri":"resource://abc"}`))
	assert.ErrorIs(t, err, gollm.ErrNotImplemented)
}
