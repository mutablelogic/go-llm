package llm

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"path"
	"strings"
	"time"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// speak is the tool which speaks text, with a model, and returns the audio,
// which is kept, as a resource of the session, so its result is a link to
// it
type speak struct {
	toolkit.Base
	models     ModelsFunc
	model      string
	voice      string
	voiceNotes VoiceNoteConverter
}

// speakInput is the input of the speak tool
type speakInput struct {
	Text         string `json:"text" help:"The text to speak, as it's said, without Markdown"`
	Voice        string `json:"voice,omitempty" help:"Voice to speak in, by its name, which the person chose, and is remembered with the model, or the model's own voice, when it's omitted" optional:""`
	Instructions string `json:"instructions,omitempty" help:"How the text is spoken, such as calmly, or cheerfully, for a model which takes them" optional:""`
	Model        string `json:"model,omitempty" help:"Model which speaks the text, by its name, such as ollama:phonon:latest, or the default, when it's omitted" optional:""`
	Name         string `json:"name,omitempty" help:"Name of the audio's file, such as morning-briefing, which its format's extension is added to, or speech, when it's omitted" optional:""`
	Title        string `json:"title,omitempty" help:"Title of the audio, for people, such as Ode to David, which it's shown, and played, with" optional:""`
	Format       string `json:"format,omitempty" enum:"voice-note" help:"voice-note, for a voice note, such as for a chat on Telegram, which is Ogg Opus, or the model's own format, when it's omitted" optional:""`
}

// speakOutput is what the speak tool returns with the audio, when it isn't
// in the format it was asked for
type speakOutput struct {
	Note string `json:"note"`
}

// named is a resource a tool returns, such as audio which was spoken, or
// text which was read, with its name, which is the one the model gave, or
// one of its own, with its format's extension, and its title, which is its
// description
type named struct {
	gollm.Resource
	name  string
	title string
}

var _ gollm.Tool = (*speak)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*speak) Name() string {
	return "speak"
}

func (t *speak) Description() string {
	return withDefault("Speaks text aloud, as audio, which is kept, and returned as a link to it, which the person can play. Use it when the person asks to hear something, rather than to read it.", t.model, "speech_output")
}

func (*speak) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[speakInput]()
}

// Timeout returns the most time the tool has, which is longer than a tool
// has by default, for speaking a long passage of text
func (*speak) Timeout() time.Duration {
	return speakTimeout
}

// Run speaks the text, with the tool's model, in the voice, or the tool's,
// and returns the audio, saying what it's doing, and the time it took
func (t *speak) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req speakInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, gollm.ErrBadParameter.With(err.Error())
	} else if req.Text = strings.TrimSpace(req.Text); req.Text == "" {
		return nil, gollm.ErrBadParameter.With("missing text")
	}
	models, model, err := modelFor(ctx, t.models, req.Model, t.model, "speech_output")
	if err != nil {
		return nil, err
	}

	// The options: the voice, or the tool's, and the instructions
	var opts []gollm.Opt
	if voice := strings.TrimSpace(req.Voice); voice != "" {
		opts = append(opts, gollm.WithOpt(gollm.VoiceKey, voice))
	} else if t.voice != "" {
		opts = append(opts, gollm.WithOpt(gollm.VoiceKey, t.voice))
	}
	if instructions := strings.TrimSpace(req.Instructions); instructions != "" {
		opts = append(opts, gollm.WithOpt(gollm.InstructionsKey, instructions))
	}

	if voice := voiceOf(opts); voice != "" {
		toolkit.Logf(ctx, "Speaking the text aloud, in the %s voice", voice)
	} else {
		toolkit.Logf(ctx, "Speaking the text aloud")
	}

	// A voice note, which the model speaks, when it can, or which is
	// encoded from what it speaks
	var spoken gollm.Resource
	var usage gollm.Usage
	format := strings.TrimSpace(req.Format)
	switch format {
	case "":
		if spoken, usage, err = models.Speak(ctx, model, req.Text, opts...); err != nil {
			return nil, err
		}
	case VoiceNote:
		var note string
		if spoken, usage, note, err = t.voiceNote(ctx, models, model, req.Text, opts); err != nil {
			toolkit.SessionFromContext(ctx).Usage(usage)
			return nil, err
		} else if note != "" {
			toolkit.SessionFromContext(ctx).Usage(usage)
			return toolkit.Result{Value: speakOutput{Note: note}, Resources: []gollm.Resource{&named{Resource: spoken, name: audioName(req.Name, spoken.Type()), title: oneLine(req.Title)}}}, nil
		}
	default:
		return nil, gollm.ErrBadParameter.Withf("format %q isn't %s", format, VoiceNote)
	}
	toolkit.SessionFromContext(ctx).Usage(usage)
	return &named{Resource: spoken, name: audioName(req.Name, spoken.Type()), title: oneLine(req.Title)}, nil
}

// voiceNote speaks the text, with the model, as a voice note, which is Ogg
// Opus: asked of the model, when it can speak one, and otherwise converted
// from what it speaks, when it rejects Opus, with the tool's converter, or,
// without one, or when what it speaks can't be, as it is, with a note which
// says why it isn't a voice note. Returns the usage of what was spoken,
// with an error, when it was spoken, and couldn't be converted.
func (t *speak) voiceNote(ctx context.Context, models Models, model gollm.Model, text string, opts []gollm.Opt) (gollm.Resource, gollm.Usage, string, error) {
	// The model's voice note, when it can speak one, such as OpenAI's
	spoken, usage, err := models.Speak(ctx, model, text, append(opts, gollm.WithOpt(gollm.AudioFormatKey, "opus"))...)
	switch {
	case err == nil && isVoiceNote(spoken.Type()):
		return &voiceNoteResource{Resource: spoken}, usage, "", nil
	case errors.Is(err, gollm.ErrBadParameter) || errors.Is(err, gollm.ErrNotImplemented):
		// Its own format, which is converted, as it rejected Opus, rather
		// than failed otherwise, such as when its provider is unavailable,
		// which another request wouldn't change
		if spoken, usage, err = models.Speak(ctx, model, text, opts...); err != nil {
			return nil, gollm.Usage{}, "", err
		}
	case err != nil:
		return nil, gollm.Usage{}, "", err
	}
	if t.voiceNotes == nil {
		return spoken, usage, "It isn't a voice note, as the model can't speak one, and the server can't encode one, without opusenc, of opus-tools", nil
	}
	toolkit.Logf(ctx, "Making it a voice note")
	encoded, err := t.voiceNotes.Convert(ctx, spoken)
	if errors.Is(err, gollm.ErrNotImplemented) {
		return spoken, usage, "It isn't a voice note, as " + err.Error(), nil
	} else if err != nil {
		// The speech's usage, which was used, as it was spoken
		return nil, usage, "", err
	}
	return encoded, usage, "", nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// voiceNoteResource is a voice note a model spoke, which is Ogg, whatever
// type it says, such as audio/opus
type voiceNoteResource struct {
	gollm.Resource
}

// Type returns the type of a voice note, which is Ogg
func (*voiceNoteResource) Type() string {
	return voiceNoteType
}

// Name returns the resource's name, with its format's extension
func (n *named) Name() string {
	return n.name
}

// Description returns the resource's title, which the model gave, which
// it's kept with, as its description, and shown with
func (n *named) Description() string {
	return n.title
}

// audioName returns the name of audio of the type, which is the name the
// model gave, as a file's name, or speech, with the type's extension, such
// as morning-briefing.wav, rather than any extension it was given
func audioName(given, contentType string) string {
	name := fileName(given)
	if name == "" {
		name = "speech"
	}
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		switch mediaType {
		case "audio/wav", "audio/x-wav", "audio/wave":
			return name + ".wav"
		case "audio/mpeg":
			return name + ".mp3"
		case "audio/l16":
			return name + ".pcm"
		case "audio/ogg", "audio/opus":
			return name + ".ogg"
		}
		if exts, _ := mime.ExtensionsByType(mediaType); len(exts) > 0 {
			return name + exts[0]
		}
	}
	return name
}

// fileName returns the name, as a file's name: its last element, without
// its extension, or characters which can't be in a file's name, such as
// slashes, or control characters, and at most 100 characters, or an empty
// string, when there's nothing left
func fileName(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if ext := path.Ext(name); ext != "" && ext != name {
		name = strings.TrimSuffix(name, ext)
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"|?*`, r) {
			return -1
		}
		return r
	}, name)
	name = strings.Trim(strings.TrimSpace(name), ".")
	if runes := []rune(name); len(runes) > 100 {
		name = strings.TrimSpace(string(runes[:100]))
	}
	return name
}

// oneLine returns the text on one line, with its runs of white space as one
// space, and at most 200 characters, as a title is
func oneLine(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > 200 {
		text = strings.TrimSpace(string(runes[:200]))
	}
	return text
}

// textFile returns the text, as a file of the type, named as the model
// gave, or else after the resource it was read from, such as scan, or else
// the fallback, with the extension, and with the title, as its description
func textFile(text, contentType, ext, given, title string, from gollm.Resource, fallback string) gollm.Resource {
	name := fileName(given)
	if name == "" && from != nil && !strings.Contains(from.Name(), "://") {
		name = fileName(from.Name())
	}
	if name == "" {
		name = fallback
	}
	file := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: contentType, Data: []byte(text)}}
	return &named{Resource: file, name: name + ext, title: oneLine(title)}
}
