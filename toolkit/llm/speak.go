package llm

import (
	"context"
	"encoding/json"
	"mime"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
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
}

// audio is the audio which was spoken, with its name, which is speech, with
// its format's extension
type audio struct {
	gollm.Resource
	name string
}

var _ gollm.Tool = (*speak)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*speak) Name() string {
	return "speak"
}

func (t *speak) Description() string {
	return withDefault("Speaks text aloud, as audio, which is kept, and returned as a link to it, which the person can play. Use it when the person asks to hear something, rather than to read it.", t.model, "speech")
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
	models, model, err := modelFor(ctx, t.models, req.Model, t.model, "speech")
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

	toolkit.Logf(ctx, "Speaking with %s", model.Name())
	spoken, usage, err := models.Speak(ctx, model, req.Text, opts...)
	if err != nil {
		return nil, err
	}
	toolkit.SessionFromContext(ctx).Usage(usage)
	return &audio{Resource: spoken, name: audioName(spoken.Type())}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Name returns the audio's name, which is speech, with its format's
// extension
func (a *audio) Name() string {
	return a.name
}

// audioName returns the name of audio of the type, which is speech, with
// its extension, such as speech.wav
func audioName(contentType string) string {
	name := "speech"
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
