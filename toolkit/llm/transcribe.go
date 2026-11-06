package llm

import (
	"context"
	"encoding/json"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// transcribe is the tool which transcribes the speech in audio, which is a
// resource, by its URI, or translates it into English, with a model, and
// returns its text, and its language
type transcribe struct {
	toolkit.Base
	models ModelsFunc
	model  string
}

// transcribeInput is the input of the transcribe tool
type transcribeInput struct {
	URI       string `json:"uri" help:"URI of the audio, which is resource://<hash>, as an attachment, or a tool's result, says"`
	Model     string `json:"model,omitempty" help:"Model which transcribes the speech, by its name, such as openai:whisper-1, or the default, when it's omitted" optional:""`
	Translate bool   `json:"translate,omitempty" help:"Whether the speech, in any language, is translated into English, rather than transcribed in its own language" optional:""`
	Language  string `json:"language,omitempty" help:"Language the speech is in, as an ISO 639-1 code, such as de, when it's known, to transcribe it better" optional:""`
}

// transcribeOutput is the text of the speech, and its language
type transcribeOutput struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
}

var _ gollm.Tool = (*transcribe)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*transcribe) Name() string {
	return "transcribe"
}

func (t *transcribe) Description() string {
	return withDefault("Transcribes the speech in audio, such as a recording, or a voice message, which is an attachment, or a tool's result, by its URI, resource://<hash>, as text, in the language it was spoken in, or translates it into English.", t.model, "transcription")
}

func (*transcribe) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[transcribeInput]()
}

// Run transcribes, or translates, the resource with the URI, in the
// session, and returns its text, and its language, saying what it's doing,
// and the seconds, and tokens, it used
func (t *transcribe) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req transcribeInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, gollm.ErrBadParameter.With(err.Error())
	}
	models, model, err := modelFor(ctx, t.models, req.Model, t.model)
	if err != nil {
		return nil, err
	}
	audio, err := toolkit.Resource(ctx, strings.TrimSpace(req.URI))
	if err != nil {
		return nil, err
	}

	// Translated, into English, or transcribed, in its language, which is
	// given to a transcription, when it's known
	var transcript gollm.Transcript
	var usage gollm.Usage
	if req.Translate {
		toolkit.Logf(ctx, "Translating %s with %s", nameOf(audio), model.Name())
		transcript, usage, err = models.Translate(ctx, model, audio, nil)
	} else {
		var opts []gollm.Opt
		if language := strings.TrimSpace(req.Language); language != "" {
			opts = append(opts, gollm.WithOpt(gollm.LanguageKey, language))
		}
		toolkit.Logf(ctx, "Transcribing %s with %s", nameOf(audio), model.Name())
		transcript, usage, err = models.Transcribe(ctx, model, audio, nil, opts...)
	}
	if err != nil {
		return nil, err
	}
	toolkit.SessionFromContext(ctx).Usage(usage)
	return transcribeOutput{Text: strings.TrimSpace(transcript.Text), Language: transcript.Language}, nil
}
