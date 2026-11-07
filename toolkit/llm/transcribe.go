package llm

import (
	"cmp"
	"context"
	"encoding/json"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	subtitle "github.com/mutablelogic/go-llm/internal/subtitle"
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
	Format    string `json:"format,omitempty" enum:"text,markdown,srt,vtt" help:"Format of a file the transcript is also kept in: text, Markdown, or subtitles, in SRT or WebVTT format, with when each part was said. With a format, a name, or a title, the transcript is kept as a file of the chat, as well as returned" optional:""`
	Name      string `json:"name,omitempty" help:"Name of the file the transcript is kept in, such as interview, which the format's extension is added to, or the recording's name, when it's omitted" optional:""`
	Title     string `json:"title,omitempty" help:"Title of the file the transcript is kept in, for people, such as Interview with Anna, which is a Markdown transcript's heading" optional:""`
}

// transcriptFormat is a format of a file a transcript is kept in: its type,
// and its extension, and whether it needs the transcript's segments, with
// when each was said
type transcriptFormat struct {
	contentType string
	ext         string
	segments    bool
}

// transcriptFormats are the formats of a file a transcript is kept in
var transcriptFormats = map[string]transcriptFormat{
	"text":     {contentType: "text/plain; charset=utf-8", ext: ".txt"},
	"markdown": {contentType: "text/markdown; charset=utf-8", ext: ".md"},
	"srt":      {contentType: "application/x-subrip; charset=utf-8", ext: ".srt", segments: true},
	"vtt":      {contentType: "text/vtt; charset=utf-8", ext: ".vtt", segments: true},
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
	models, model, err := modelFor(ctx, t.models, req.Model, t.model, "transcription")
	if err != nil {
		return nil, err
	}
	// The format of the file it's kept in, when it's kept in one, which is
	// text, unless it's given
	keep := strings.TrimSpace(req.Format) != "" || strings.TrimSpace(req.Name) != "" || strings.TrimSpace(req.Title) != ""
	format, ok := transcriptFormats[strings.ToLower(strings.TrimSpace(cmp.Or(req.Format, "text")))]
	if !ok {
		return nil, gollm.ErrBadParameter.Withf("unknown format %q, which isn't text, markdown, srt or vtt", req.Format)
	}
	audio, err := toolkit.Resource(ctx, strings.TrimSpace(req.URI))
	if err != nil {
		return nil, err
	}

	// Translated, into English, or transcribed, in its language, which is
	// given to a transcription, when it's known, with its segments, when
	// its format needs them
	var transcript gollm.Transcript
	var usage gollm.Usage
	var opts []gollm.Opt
	if keep && format.segments {
		opts = append(opts, gollm.WithOpt(gollm.SegmentsKey, true))
	}
	if req.Translate {
		toolkit.Logf(ctx, "Translating what's said in %s into English", describe(audio))
		transcript, usage, err = models.Translate(ctx, model, audio, nil, opts...)
	} else {
		if language := strings.TrimSpace(req.Language); language != "" {
			opts = append(opts, gollm.WithOpt(gollm.LanguageKey, language))
		}
		toolkit.Logf(ctx, "Transcribing what's said in %s", describe(audio))
		transcript, usage, err = models.Transcribe(ctx, model, audio, nil, opts...)
	}
	if err != nil {
		return nil, err
	}
	toolkit.SessionFromContext(ctx).Usage(usage)
	output := transcribeOutput{Text: strings.TrimSpace(transcript.Text), Language: transcript.Language}
	if !keep {
		return output, nil
	}
	content, err := transcriptContent(req.Format, oneLine(req.Title), transcript)
	if err != nil {
		return nil, err
	}
	file := textFile(content, format.contentType, format.ext, req.Name, req.Title, audio, "transcript")
	return toolkit.Result{Value: output, Resources: []gollm.Resource{file}}, nil
}

// transcriptContent returns the transcript, in the format: its text, as
// Markdown, with its title, as its heading, and who said what, when it's
// known, or as subtitles, from its segments, or its text, when it has none
func transcriptContent(format, title string, transcript gollm.Transcript) (string, error) {
	var b strings.Builder
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "srt", "vtt":
		// Subtitles need when each part was said, which a model which
		// returns only text hasn't, rather than a cue which is never shown
		if len(transcript.Segments) == 0 {
			return "", gollm.ErrBadParameter.Withf("the model returned no timings, so the transcript can't be %s subtitles, but can be text, or markdown", strings.ToUpper(strings.TrimSpace(format)))
		}
		if err := subtitle.Write(&b, subtitle.Format(strings.ToLower(strings.TrimSpace(format))), transcript.Segments); err != nil {
			return "", err
		}
	case "markdown":
		if title != "" {
			b.WriteString("# " + title + "\n\n")
		}
		b.WriteString(markdownTranscript(transcript))
		b.WriteString("\n")
	default:
		b.WriteString(strings.TrimSpace(transcript.Text) + "\n")
	}
	return b.String(), nil
}

// markdownTranscript returns the transcript as Markdown: a paragraph for
// each turn of who spoke, with their name, in bold, when the segments say
// who spoke them, or otherwise its text
func markdownTranscript(transcript gollm.Transcript) string {
	speakers := false
	for _, segment := range transcript.Segments {
		if segment.Speaker != "" {
			speakers = true
			break
		}
	}
	if !speakers {
		return strings.TrimSpace(transcript.Text)
	}
	var paragraphs []string
	var speaker string
	var words []string
	flush := func() {
		if len(words) > 0 {
			paragraphs = append(paragraphs, "**"+cmp.Or(speaker, "Unknown")+":** "+strings.Join(words, " "))
		}
		words = nil
	}
	for _, segment := range transcript.Segments {
		if segment.Speaker != speaker {
			flush()
			speaker = segment.Speaker
		}
		if text := strings.TrimSpace(segment.Text); text != "" {
			words = append(words, text)
		}
	}
	flush()
	return strings.Join(paragraphs, "\n\n")
}
