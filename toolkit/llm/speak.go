package llm

import (
	"context"
	"encoding/json"
	"mime"
	"path"
	"strings"

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
	models ModelsFunc
	model  string
	voice  string
}

// speakInput is the input of the speak tool
type speakInput struct {
	Text         string `json:"text" help:"The text to speak, as it's said, without Markdown"`
	Voice        string `json:"voice,omitempty" help:"Voice to speak in, by its name, which the person chose, and is remembered with the model, or the model's own voice, when it's omitted" optional:""`
	Instructions string `json:"instructions,omitempty" help:"How the text is spoken, such as calmly, or cheerfully, for a model which takes them" optional:""`
	Model        string `json:"model,omitempty" help:"Model which speaks the text, by its name, such as ollama:phonon:latest, or the default, when it's omitted" optional:""`
	Name         string `json:"name,omitempty" help:"Name of the audio's file, such as morning-briefing, which its format's extension is added to, or speech, when it's omitted" optional:""`
	Title        string `json:"title,omitempty" help:"Title of the audio, for people, such as Ode to David, which it's shown, and played, with" optional:""`
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
	spoken, usage, err := models.Speak(ctx, model, req.Text, opts...)
	if err != nil {
		return nil, err
	}
	toolkit.SessionFromContext(ctx).Usage(usage)
	return &named{Resource: spoken, name: audioName(req.Name, spoken.Type()), title: oneLine(req.Title)}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

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
