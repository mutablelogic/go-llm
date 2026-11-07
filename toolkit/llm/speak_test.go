package llm_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"uuid"

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
	opts       *gollm.Opts
	noSegments bool // Whether it never returns segments, as some models don't
}

type model struct {
	name string
	caps gollm.ModelCap
}

func (m model) Name() string                   { return m.name }
func (model) Provider() gollm.Provider         { return nil }
func (model) Description() string              { return "" }
func (model) Created() time.Time               { return time.Time{} }
func (model) Aliases() []string                { return nil }
func (model) Meta() map[string]json.RawMessage { return nil }
func (model) InputTokenLimit() *uint           { return nil }
func (model) OutputTokenLimit() *uint          { return nil }
func (m model) Capabilities() gollm.ModelCap   { return m.caps }

func (m *models) ListModels(_ context.Context, providers ...string) ([]gollm.Model, error) {
	if len(providers) > 0 && providers[0] != "fake" {
		return nil, gollm.ErrNotFound.Withf("provider %q", providers[0])
	}
	yes := true
	return []gollm.Model{
		model{name: "chat", caps: gollm.ModelCap{Completion: &yes, Tools: &yes}},
		model{name: "phonon", caps: gollm.ModelCap{Speech: &yes}},
		model{name: "reader", caps: gollm.ModelCap{OCR: &yes, Vision: &yes}},
		model{name: "whisper", caps: gollm.ModelCap{Transcription: &yes, Translation: &yes}},
	}, nil
}

func (m *models) Name(model gollm.Model) string {
	return "fake:" + model.Name()
}

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
	if segments, _ := gollm.GetOpt[bool](o, gollm.SegmentsKey); segments && !m.noSegments {
		return gollm.Transcript{Text: "Bonjour. Salut. Ça va?", Language: "fr", Segments: []gollm.TranscriptSegment{
			{Start: 0, End: 1500 * time.Millisecond, Text: "Bonjour.", Speaker: "A"},
			{Start: 1500 * time.Millisecond, End: 3 * time.Second, Text: "Salut.", Speaker: "A"},
			{Start: 3 * time.Second, End: 4 * time.Second, Text: "Ça va?", Speaker: "B"},
		}}, gollm.Usage{}, nil
	}
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
	require.Len(t, ns.Tools(), 4)
	assert.Contains(t, ns.Tools()[0].Description(), "recall the memory preferred_models")
	assert.Contains(t, ns.Tools()[0].Description(), "with the speech capability with the models tool")
	_, err = ns.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello"}`))
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	assert.ErrorContains(t, err, "the memory preferred_models, or list the models with the speech capability", "it says how to choose one")
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

	// A voice, without a model, is the default for the model which is given
	voiced, err := llm.New(func() llm.Models { return m }, llm.WithSpeak("", "Freya"))
	require.NoError(t, err)
	_, err = voiced.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello","model":"phonon"}`))
	require.NoError(t, err)
	voice, _ = gollm.GetOpt[string](m.opts, gollm.VoiceKey)
	assert.Equal(t, "Freya", voice)
	_, err = llm.New(func() llm.Models { return m }, llm.WithSpeak(" ", ""))
	assert.ErrorIs(t, err, gollm.ErrBadParameter)

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

func Test_Models(t *testing.T) {
	ctx := context.Background()
	ns, err := llm.New(func() llm.Models { return &models{} })
	require.NoError(t, err)
	tool := ns.Tools()[3]
	assert.Equal(t, "models", tool.Name())
	assert.Contains(t, ns.Tools()[1].Description(), "list the models with the ocr capability with the models tool", "ocr says how to find a model")
	run := func(input string) (map[string]any, error) {
		result, err := tool.Run(ctx, json.RawMessage(input))
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(result)
		require.NoError(t, err)
		var output map[string]any
		require.NoError(t, json.Unmarshal(data, &output))
		return output, nil
	}
	names := func(output map[string]any) []string {
		var result []string
		for _, m := range output["models"].([]any) {
			result = append(result, m.(map[string]any)["name"].(string))
		}
		return result
	}

	// Every model, or those with every one of the capabilities, by the names
	// they're given to tools by
	output, err := run(`{}`)
	require.NoError(t, err)
	assert.Equal(t, float64(4), output["count"])
	output, err = run(`{"capabilities":["transcription","translation"]}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"fake:whisper"}, names(output))
	assert.Equal(t, []any{"transcription", "translation"}, output["models"].([]any)[0].(map[string]any)["capabilities"])
	output, err = run(`{"capabilities":["ocr"],"provider":"fake"}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"fake:reader"}, names(output))
	output, err = run(`{"capabilities":["speech","vision"]}`)
	require.NoError(t, err)
	assert.Equal(t, float64(0), output["count"])
	assert.Equal(t, []any{}, output["models"])

	// A capability which isn't one, a provider which isn't there, and no
	// models yet
	_, err = run(`{"capabilities":["telepathy"]}`)
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	_, err = run(`{"provider":"missing"}`)
	assert.ErrorIs(t, err, gollm.ErrNotFound)
	none, err := llm.New(func() llm.Models { return nil })
	require.NoError(t, err)
	_, err = none.Tools()[3].Run(ctx, json.RawMessage(`{}`))
	assert.ErrorIs(t, err, gollm.ErrServiceUnavailable)

	// A capability which isn't one is said to be, before the models are
	// asked, even when they can't be listed
	_, err = none.Tools()[3].Run(ctx, json.RawMessage(`{"capabilities":["telepathy"]}`))
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	_, err = run(`{"capabilities":["telepathy"],"provider":"missing"}`)
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
}

func Test_Logs(t *testing.T) {
	// What the tools say they're doing is English, with what they're doing
	// it to, rather than the model's name, or a resource's URI
	ctx := context.Background()
	m := &models{}
	ns, err := llm.New(func() llm.Models { return m }, llm.WithSpeak("phonon", ""), llm.WithOCR("phonon"), llm.WithTranscribe("phonon"))
	require.NoError(t, err)
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(ns))
	var lines []string
	session := toolkit.NewSession("llm", uuid.Nil(), func(what gollm.StreamType, text string) {
		if what == gollm.StreamLog {
			lines = append(lines, strings.TrimSpace(text))
		}
	}, nil)
	ctx = toolkit.WithResolver(toolkit.WithSession(ctx, session), func(_ context.Context, uri string) (gollm.Resource, error) {
		types := map[string]string{"resource://pdf": "application/pdf", "resource://wav": "audio/wav"}
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: types[uri], Data: []byte("x")}}, nil
	})
	for _, call := range []struct{ tool, input string }{
		{"llm:speak", `{"text":"Hello","voice":"Zoey"}`},
		{"llm:speak", `{"text":"Hello"}`},
		{"llm:ocr", `{"uri":"resource://pdf"}`},
		{"llm:ocr", `{"uri":"resource://pdf","pages":"0-1"}`},
		{"llm:transcribe", `{"uri":"resource://wav"}`},
		{"llm:transcribe", `{"uri":"resource://wav","translate":true}`},
		{"llm:models", `{"capabilities":["ocr","speech"]}`},
		{"llm:models", `{"capabilities":["vision","tools","speech"]}`},
		{"llm:models", `{"capabilities":["image"]}`},
	} {
		_, err := tk.CallTool(ctx, call.tool, json.RawMessage(call.input))
		require.NoError(t, err, call.tool)
	}
	assert.Equal(t, []string{
		"Speaking the text aloud, in the Zoey voice",
		"Speaking the text aloud",
		"Reading the text in the PDF",
		"Reading the text on pages 0-1 of the PDF",
		"Transcribing what's said in the recording",
		"Translating what's said in the recording into English",
		"Looking for models which can read documents and speak",
		"Looking for models which can see images, call tools and speak",
		"Looking for models which can make images",
	}, lines)
}

func Test_SpeakName(t *testing.T) {
	// The audio is named as the model says, as a file's name, with its
	// format's extension, or speech
	ctx := context.Background()
	ns, err := llm.New(func() llm.Models { return &models{} }, llm.WithSpeak("phonon", ""))
	require.NoError(t, err)
	tool := ns.Tools()[0]
	for name, want := range map[string]string{
		"":                       "speech.wav",
		"Morning briefing":       "Morning briefing.wav",
		"morning-briefing.mp3":   "morning-briefing.wav",
		"../../etc/passwd":       "passwd.wav",
		`C:\\temp\\ode.wav`:      "ode.wav",
		"  what? <now>  ":        "what now.wav",
		"...":                    "speech.wav",
		"\x00\x01":               "speech.wav",
		strings.Repeat("a", 150): strings.Repeat("a", 100) + ".wav",
	} {
		input, err := json.Marshal(map[string]string{"text": "Hello", "name": name})
		require.NoError(t, err)
		result, err := tool.Run(ctx, input)
		require.NoError(t, err, name)
		assert.Equal(t, want, result.(gollm.Resource).Name(), name)
	}
}

func Test_SpeakTitle(t *testing.T) {
	// The audio has the title the model gave, on one line, as its
	// description, which it's kept, and shown, with
	ctx := context.Background()
	ns, err := llm.New(func() llm.Models { return &models{} }, llm.WithSpeak("phonon", ""))
	require.NoError(t, err)
	result, err := ns.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello","name":"ode","title":"  Ode to\n David  "}`))
	require.NoError(t, err)
	audio := result.(gollm.Resource)
	assert.Equal(t, "ode.wav", audio.Name())
	assert.Equal(t, "Ode to David", audio.Description())
	result, err = ns.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello"}`))
	require.NoError(t, err)
	assert.Empty(t, result.(gollm.Resource).Description())
}

func Test_TextFiles(t *testing.T) {
	// With a format, a name, or a title, the text which is read, or the
	// transcript, is kept as a file, as well as returned
	ctx := toolkit.WithResolver(context.Background(), func(_ context.Context, uri string) (gollm.Resource, error) {
		types := map[string]string{"resource://pdf": "application/pdf", "resource://wav": "audio/wav"}
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: types[uri], Data: []byte("Report")}}, nil
	})
	m := &models{}
	ns, err := llm.New(func() llm.Models { return m }, llm.WithOCR("phonon"), llm.WithTranscribe("phonon"))
	require.NoError(t, err)
	ocr, transcribe := ns.Tools()[1], ns.Tools()[2]
	file := func(result any) (gollm.Resource, string) {
		t.Helper()
		r, ok := result.(toolkit.Result)
		require.True(t, ok, "a result with a file")
		require.Len(t, r.Resources, 1)
		data, err := gollm.ReadAll(ctx, r.Resources[0])
		require.NoError(t, err)
		return r.Resources[0], string(data)
	}

	// Without them, only the text
	result, err := transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://wav"}`))
	require.NoError(t, err)
	_, isResult := result.(toolkit.Result)
	assert.False(t, isResult)

	// Subtitles, with when each part was said
	result, err = transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://wav","format":"srt"}`))
	require.NoError(t, err)
	f, content := file(result)
	assert.Equal(t, "transcript.srt", f.Name())
	assert.Equal(t, "application/x-subrip; charset=utf-8", f.Type())
	assert.Empty(t, f.Description())
	assert.Contains(t, content, "00:00:00,000 --> ")
	result, err = transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://wav","format":"vtt","name":"chat"}`))
	require.NoError(t, err)
	f, content = file(result)
	assert.Equal(t, "chat.vtt", f.Name())
	assert.Equal(t, "text/vtt; charset=utf-8", f.Type())
	assert.True(t, strings.HasPrefix(content, "WEBVTT"))

	// Markdown, with its title, as its heading, which doesn't ask for
	// segments, as some models reject them, so it's the text
	result, err = transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://wav","format":"markdown","title":"A chat"}`))
	require.NoError(t, err)
	f, content = file(result)
	assert.Equal(t, "transcript.md", f.Name())
	assert.Equal(t, "text/markdown; charset=utf-8", f.Type())
	assert.Equal(t, "A chat", f.Description())
	assert.Equal(t, "# A chat\n\nBonjour\n", content)
	asked, _ := gollm.GetOpt[bool](m.opts, gollm.SegmentsKey)
	assert.False(t, asked, "markdown doesn't ask for segments")

	// Subtitles, from a model which returns no timings, aren't kept, as
	// they'd never be shown
	m.noSegments = true
	_, err = transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://wav","format":"srt"}`))
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	assert.ErrorContains(t, err, "no timings")
	m.noSegments = false

	// Text, with a name
	result, err = transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://wav","name":"interview"}`))
	require.NoError(t, err)
	f, content = file(result)
	assert.Equal(t, "interview.txt", f.Name())
	assert.Equal(t, "text/plain; charset=utf-8", f.Type())
	assert.Equal(t, "Bonjour\n", content)

	// A format which isn't one
	_, err = transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://wav","format":"docx"}`))
	assert.ErrorIs(t, err, gollm.ErrBadParameter)

	// The text of a document, as Markdown, with its title
	result, err = ocr.Run(ctx, json.RawMessage(`{"uri":"resource://pdf","title":"The report"}`))
	require.NoError(t, err)
	f, content = file(result)
	assert.Equal(t, "text.md", f.Name())
	assert.Equal(t, "text/markdown; charset=utf-8", f.Type())
	assert.Equal(t, "The report", f.Description())
	assert.Contains(t, content, "# Report")
	result, err = ocr.Run(ctx, json.RawMessage(`{"uri":"resource://pdf"}`))
	require.NoError(t, err)
	_, isResult = result.(toolkit.Result)
	assert.False(t, isResult)
}
