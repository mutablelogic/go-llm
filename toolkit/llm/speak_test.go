package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	images     []gollm.Resource // The images it was last asked to change
	noSegments bool             // Whether it never returns segments, as some models don't
	text       string           // The transcript's text, when it's set
	opus       string           // How it speaks Opus: "native", "vorbis", "reject", "unavailable", or, when it's empty, as WAV
	usage      gollm.Usage      // The usage of what it speaks
	spoken     int              // How many times it spoke
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
		model{name: "chat", caps: gollm.ModelCap{Chat: &yes, Tools: &yes}},
		model{name: "phonon", caps: gollm.ModelCap{SpeechOutput: &yes}},
		model{name: "reader", caps: gollm.ModelCap{OCR: &yes, ImageInput: &yes}},
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
	m.spoken++
	if format, _ := gollm.GetOpt[string](o, gollm.AudioFormatKey); format == "opus" {
		switch m.opus {
		case "native":
			return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/ogg", Data: []byte("OggS" + text)}}, m.usage, nil
		case "vorbis":
			return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/ogg; codecs=vorbis", Data: []byte("OggS" + text)}}, m.usage, nil
		case "reject":
			return nil, gollm.Usage{}, gollm.ErrBadParameter.Withf("audio format %q isn't wav or pcm", format)
		case "unavailable":
			return nil, gollm.Usage{}, gollm.ErrServiceUnavailable.With("429 Too Many Requests")
		}
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wav", Data: []byte(text)}}, m.usage, nil
}

func (m *models) ListVoices(_ context.Context, model gollm.Model, opts ...gollm.Opt) ([]gollm.Voice, string, error) {
	if model.Name() != "phonon" {
		return nil, "", gollm.ErrNotImplemented.Withf("%s doesn't speak", model.Name())
	}
	voices := make([]gollm.Voice, 30)
	for i := range voices {
		voices[i] = gollm.Voice{ID: fmt.Sprintf("voice-%d", i), Title: fmt.Sprintf("Voice %d", i)}
	}
	return gollm.Paged(voices, opts...)
}

func (m *models) Illustrate(_ context.Context, _ gollm.Model, prompt string, images []gollm.Resource, opts ...gollm.Opt) ([]gollm.Resource, gollm.Usage, error) {
	o, err := gollm.ApplyOpts(opts...)
	if err != nil {
		return nil, gollm.Usage{}, err
	}
	m.opts, m.images = o, images
	made := []gollm.Resource{&providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/png", Data: []byte(prompt)}}}
	if strings.Contains(prompt, "two") {
		made = append(made, &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/jpeg", Data: []byte(prompt)}})
	}
	return made, gollm.Usage{OutputTokens: 1290}, nil
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
	if m.text != "" {
		return gollm.Transcript{Text: m.text, Language: "en"}, gollm.Usage{}, nil
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
	require.Len(t, ns.Tools(), 5)
	assert.Contains(t, ns.Tools()[0].Description(), "recall the memory preferred_models")
	assert.Contains(t, ns.Tools()[0].Description(), "with the speech_output capability with the models tool")
	_, err = ns.Tools()[0].Run(ctx, json.RawMessage(`{"text":"Hello"}`))
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	assert.ErrorContains(t, err, "the memory preferred_models, or list the models with the speech_output capability", "it says how to choose one")
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
	resolved := toolkit.WithResolver(ctx, each(func(string) (gollm.Resource, error) {
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wav", Data: []byte("x")}}, nil
	}))
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
	resolved := toolkit.WithResolver(ctx, each(func(uri string) (gollm.Resource, error) {
		if uri != "resource://abc" {
			return nil, gollm.ErrNotFound.Withf("resource %s", uri)
		}
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "application/pdf", Data: []byte("Report")}}, nil
	}))

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
	output, err = run(`{"capabilities":["speech_output","image_input"]}`)
	require.NoError(t, err)
	assert.Equal(t, float64(0), output["count"])
	assert.Equal(t, []any{}, output["models"])

	// Models which speak have their voices, when they're asked for, a page
	// at a time, and not otherwise, as there can be many
	output, err = run(`{"capabilities":["speech_output"]}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"fake:phonon"}, names(output))
	phonon := output["models"].([]any)[0].(map[string]any)
	require.Len(t, phonon["voices"], 25)
	assert.Equal(t, map[string]any{"id": "voice-0", "title": "Voice 0"}, phonon["voices"].([]any)[0])
	assert.Equal(t, "25", phonon["voices_next"])
	output, err = run(`{}`)
	require.NoError(t, err)
	for _, m := range output["models"].([]any) {
		assert.NotContains(t, m.(map[string]any), "voices")
	}

	// The next page of a model's voices, which is the last
	output, err = run(`{"model":"fake:phonon","voices_next":"25"}`)
	require.NoError(t, err)
	phonon = output["models"].([]any)[0].(map[string]any)
	require.Len(t, phonon["voices"], 5)
	assert.Equal(t, map[string]any{"id": "voice-25", "title": "Voice 25"}, phonon["voices"].([]any)[0])
	assert.NotContains(t, phonon, "voices_next")

	// A model by its name, the next page without a model, a token which
	// isn't one, and the next page of a model which doesn't speak
	output, err = run(`{"model":"fake:reader"}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"fake:reader"}, names(output))
	_, err = run(`{"voices_next":"25"}`)
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	_, err = run(`{"model":"fake:phonon","voices_next":"nonsense"}`)
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	_, err = run(`{"model":"fake:chat","voices_next":"25"}`)
	assert.ErrorIs(t, err, gollm.ErrNotImplemented)

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
	ctx = toolkit.WithResolver(toolkit.WithSession(ctx, session), each(func(uri string) (gollm.Resource, error) {
		types := map[string]string{"resource://pdf": "application/pdf", "resource://wav": "audio/wav"}
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: types[uri], Data: []byte("x")}}, nil
	}))
	for _, call := range []struct{ tool, input string }{
		{"llm:speak", `{"text":"Hello","voice":"Zoey"}`},
		{"llm:speak", `{"text":"Hello"}`},
		{"llm:ocr", `{"uri":"resource://pdf"}`},
		{"llm:ocr", `{"uri":"resource://pdf","pages":"0-1"}`},
		{"llm:transcribe", `{"uri":"resource://wav"}`},
		{"llm:transcribe", `{"uri":"resource://wav","translate":true}`},
		{"llm:models", `{"capabilities":["ocr","speech_output"]}`},
		{"llm:models", `{"capabilities":["image_input","tools","speech_output"]}`},
		{"llm:models", `{"capabilities":["image_output"]}`},
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
		"Looking for models which can recognize text and speak",
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
	ctx := toolkit.WithResolver(context.Background(), each(func(uri string) (gollm.Resource, error) {
		types := map[string]string{"resource://pdf": "application/pdf", "resource://wav": "audio/wav"}
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: types[uri], Data: []byte("Report")}}, nil
	}))
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

func Test_Illustrate(t *testing.T) {
	m := &models{}
	ns, err := llm.New(func() llm.Models { return m })
	require.NoError(t, err)
	tool := ns.Tools()[4]
	assert.Equal(t, "illustrate", tool.Name())
	assert.Contains(t, tool.Description(), "with the image_output capability with the models tool")
	var lines []string
	session := toolkit.NewSession("llm", uuid.Nil(), func(what gollm.StreamType, text string) {
		if what == gollm.StreamLog {
			lines = append(lines, strings.TrimSpace(text))
		}
	}, nil)
	ctx := toolkit.WithResolver(toolkit.WithSession(context.Background(), session), each(func(uri string) (gollm.Resource, error) {
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "image/png", Data: []byte(uri)}}, nil
	}))
	run := func(input string) (toolkit.Result, error) {
		result, err := tool.Run(ctx, json.RawMessage(input))
		if err != nil {
			return toolkit.Result{}, err
		}
		return result.(toolkit.Result), nil
	}

	// An image, named, and titled, as the model says, with its aspect ratio
	result, err := run(`{"prompt":"A banana","model":"phonon","name":"banana","title":"A banana on a beach","aspect_ratio":"16:9"}`)
	require.NoError(t, err)
	require.Len(t, result.Resources, 1)
	assert.Equal(t, "banana.png", result.Resources[0].Name())
	assert.Equal(t, "A banana on a beach", result.Resources[0].Description())
	ratio, _ := gollm.GetOpt[string](m.opts, gollm.AspectRatioKey)
	assert.Equal(t, "16:9", ratio)

	// Images it changes, by their URIs, and more than one, which are numbered
	result, err = run(`{"prompt":"two bananas","model":"phonon","images":["resource://a","resource://b"]}`)
	require.NoError(t, err)
	require.Len(t, m.images, 2)
	require.Len(t, result.Resources, 2)
	assert.Equal(t, "image-1.png", result.Resources[0].Name())
	assert.Equal(t, "image-2.jpg", result.Resources[1].Name())
	_, err = run(`{"prompt":"Sunset","model":"phonon","images":["resource://a"]}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"Making an image", "Combining 2 images", "Changing the image"}, lines)

	// No prompt, and no model, which the person chooses
	_, err = run(`{"prompt":" ","model":"phonon"}`)
	assert.ErrorIs(t, err, gollm.ErrBadParameter)
	_, err = run(`{"prompt":"A banana"}`)
	assert.ErrorContains(t, err, "image_output capability")
}

// each returns the function which resolves each URI with the function
func each(fn func(uri string) (gollm.Resource, error)) toolkit.ResolveFunc {
	return func(_ context.Context, uris ...string) ([]gollm.Resource, error) {
		result := make([]gollm.Resource, 0, len(uris))
		for _, uri := range uris {
			resource, err := fn(uri)
			if err != nil {
				return nil, err
			}
			result = append(result, resource)
		}
		return result, nil
	}
}

func Test_Timeouts(t *testing.T) {
	// The tools which ask a model to do something which takes a while have
	// longer than a tool has by default, and listing models doesn't
	ns, err := llm.New(func() llm.Models { return &models{} })
	require.NoError(t, err)
	want := map[string]time.Duration{"models": 0, "speak": 2 * time.Minute, "ocr": 5 * time.Minute, "transcribe": 5 * time.Minute, "illustrate": 2 * time.Minute}
	got := map[string]time.Duration{}
	for _, tool := range ns.Tools() {
		got[tool.Name()] = tool.Timeout()
	}
	assert.Equal(t, want, got)
}

func Test_LongTranscript(t *testing.T) {
	// A long transcript is kept as a file, whatever's asked, and its start
	// is returned, ending at a word, with a note to read the rest with the
	// resource tool
	ctx := toolkit.WithResolver(context.Background(), each(func(string) (gollm.Resource, error) {
		return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/mpeg", Data: []byte("ID3")}}, nil
	}))
	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 400)
	m := &models{text: strings.TrimSpace(text)}
	ns, err := llm.New(func() llm.Models { return m }, llm.WithTranscribe("phonon"))
	require.NoError(t, err)
	transcribe := ns.Tools()[2]
	result, err := transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://mp3"}`))
	require.NoError(t, err)
	r, ok := result.(toolkit.Result)
	require.True(t, ok, "a result with a file")
	require.Len(t, r.Resources, 1)
	data, err := gollm.ReadAll(ctx, r.Resources[0])
	require.NoError(t, err)
	assert.Equal(t, m.text, strings.TrimSpace(string(data)), "the file has the whole of it")
	output, err := json.Marshal(r.Value)
	require.NoError(t, err)
	var got struct {
		Text      string `json:"text"`
		Truncated bool   `json:"truncated"`
		Note      string `json:"note"`
	}
	require.NoError(t, json.Unmarshal(output, &got))
	assert.True(t, got.Truncated)
	assert.LessOrEqual(t, len(got.Text), 8000+len("…"))
	assert.True(t, strings.HasSuffix(got.Text, "…"))
	assert.True(t, strings.HasPrefix(m.text, strings.TrimSuffix(got.Text, "…")))
	assert.NotContains(t, got.Text, "jum…", "it ends at a word")
	assert.Contains(t, got.Note, "llm:resource")

	// A short one is returned whole, without a file
	m.text = "Hello."
	result, err = transcribe.Run(ctx, json.RawMessage(`{"uri":"resource://mp3"}`))
	require.NoError(t, err)
	_, isResult := result.(toolkit.Result)
	assert.False(t, isResult)
	assert.Contains(t, transcribe.Description(), "llm:resource")
}
