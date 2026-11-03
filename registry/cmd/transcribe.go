package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type TranscribeCmd struct {
	File   string                     `arg:"" name:"file" type:"existingfile" help:"The audio, whose speech is transcribed."`
	Model  string                     `name:"model" help:"Model name, or provider and model name separated by a colon, such as openai:whisper-1. Defaults to the model last used for transcribing speech." optional:""`
	Opt    map[string]json.RawMessage `name:"opt" help:"Options, by key, such as language=fr, segments=true, or speakers=true, which can be given more than once." optional:""`
	Stream bool                       `name:"stream" negatable:"" default:"true" help:"Print the transcript as it's transcribed, or with --no-stream, once it's complete."`
	Format string                     `name:"format" enum:"text,srt,vtt" default:"text" help:"Print the transcript as text, or its segments, or as subtitles, in SRT or WebVTT format, which are its segments, as cues, ending when the speaker changes, or after a silence (${enum})."`
}

type TranslateCmd struct {
	File   string                     `arg:"" name:"file" type:"existingfile" help:"The audio, whose speech, in any language, is translated into English."`
	Model  string                     `name:"model" help:"Model name, or provider and model name separated by a colon, such as openai:whisper-1. Defaults to the model last used for translating speech." optional:""`
	Opt    map[string]json.RawMessage `name:"opt" help:"Options, by key, such as segments=true, which can be given more than once." optional:""`
	Stream bool                       `name:"stream" negatable:"" default:"true" help:"Print the transcript as it's translated, or with --no-stream, once it's complete."`
	Format string                     `name:"format" enum:"text,srt,vtt" default:"text" help:"Print the transcript as text, or its segments, or as subtitles, in SRT or WebVTT format, which are its segments, as cues, ending after a silence (${enum})."`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The keys of the default models for transcribing, and translating, speech,
// which are the models last used for them, as few models can, and fewer
// can translate
const (
	defaultTranscribeModelKey = "llm.transcribe_model"
	defaultTranslateModelKey  = "llm.translate_model"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the text of the speech in the file, or its segments, when
// they're asked for, or the response, with debugging, and keeps the model,
// by its canonical name, as the default for transcribing speech
func (cmd *TranscribeCmd) Run(ctx server.Cmd) error {
	return runTranscribe(ctx, "llm.TranscribeCmd", defaultTranscribeModelKey, cmd.File, cmd.Model, cmd.Opt, cmd.Stream, cmd.Format, (*httpclient.Client).Transcribe)
}

// Run prints the English text of the speech in the file, as TranscribeCmd
// does, and keeps the model as the default for translating speech
func (cmd *TranslateCmd) Run(ctx server.Cmd) error {
	return runTranscribe(ctx, "llm.TranslateCmd", defaultTranslateModelKey, cmd.File, cmd.Model, cmd.Opt, cmd.Stream, cmd.Format, (*httpclient.Client).Translate)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// runTranscribe transcribes, or translates, the speech in the file, with the
// model, or the default model, by its key, which is the model once it's
// transcribed the speech, with the client's method, and prints it in the
// format as it's transcribed, when it's streamed. Subtitles are segments,
// so they're asked for, unless segments, or speakers, are.
func runTranscribe(ctx server.Cmd, name, key, file, model string, opt map[string]json.RawMessage, stream bool, format string, fn func(*httpclient.Client, context.Context, schema.TranscribeRequest, llm.TranscriberStreamFn) (*schema.TranscribeResponse, error)) error {
	// The model, or the model last used
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(ctx.GetString(key))
	}
	if model == "" {
		return fmt.Errorf("missing model: set one with --model, and it's used until another is set")
	}

	// The audio
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	// The transcript is printed as it's transcribed: its text, or its
	// segments, when they're asked for, unless it's debugged
	debug := ctx.IsDebug()
	req := schema.TranscribeRequest{
		Model: model,
		Type:  providerschema.ResourceType(data, filepath.Base(file)),
		Data:  data,
		Opt:   quoteOpts(opt),
	}
	opts, err := llm.ApplyOpts(req.Opts()...)
	if err != nil {
		return err
	}
	segments, _ := llm.GetOpt[bool](opts, llm.SegmentsKey)
	speakers, _ := llm.GetOpt[bool](opts, llm.SpeakersKey)
	if format != "text" && !segments && !speakers {
		if req.Opt == nil {
			req.Opt = make(map[string]json.RawMessage, 1)
		}
		req.Opt[llm.SegmentsKey], segments = json.RawMessage(`true`), true
	}
	printer := newTranscriptPrinter(os.Stdout, format, segments || speakers)
	var streamFn llm.TranscriberStreamFn
	if stream && !debug {
		streamFn = printer.Stream
	}

	return withClient(ctx, name, func(reqctx context.Context, client *httpclient.Client) error {
		response, err := fn(client, reqctx, req, streamFn)
		if err != nil {
			return err
		}

		// The model is the default once it's transcribed the speech, by its
		// canonical name, as its name could match another provider's model
		// later
		if err := ctx.Set(key, response.Model); err != nil {
			return err
		}

		// With debugging, the response, or the transcript, once it's
		// streamed, or otherwise each segment, or the text
		switch {
		case debug:
			fmt.Println(response)
		case printer.streamed:
			printer.Close()
		default:
			if printer.segments {
				for i := range response.Segments {
					printer.Stream("", &response.Segments[i])
				}
			} else {
				printer.Stream(response.Text, nil)
			}
			printer.Close()
		}
		return nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// TRANSCRIPT PRINTER

// transcriptPrinter prints a transcript as it's transcribed: each piece of
// its text, or, when segments are asked for, each segment, on a line, with
// when it was spoken, and who spoke it, or its segments as subtitles
type transcriptPrinter struct {
	w         io.Writer
	segments  bool            // Whether segments are printed, rather than text
	subtitles *subtitleWriter // The subtitles, when the segments are printed as them
	streamed  bool            // Whether anything has been printed
	text      bool            // Whether text has been printed, without a newline
}

// newTranscriptPrinter returns a printer of a transcript to the writer, in
// the format, which is text, or subtitles, in srt or vtt format, which are
// segments, and of its segments, rather than its text, when segments is true
func newTranscriptPrinter(w io.Writer, format string, segments bool) *transcriptPrinter {
	p := &transcriptPrinter{w: w, segments: segments}
	if format == "srt" || format == "vtt" {
		p.segments, p.subtitles = true, newSubtitleWriter(w, format == "vtt")
	}
	return p
}

// Stream prints a piece of the text, or a segment
func (p *transcriptPrinter) Stream(text string, segment *llm.TranscriptSegment) {
	switch {
	case p.subtitles != nil && segment != nil:
		p.subtitles.Add(*segment)
		p.streamed = true
	case p.segments && segment != nil:
		speaker := ""
		if segment.Speaker != "" {
			speaker = segment.Speaker + ": "
		}
		fmt.Fprintf(p.w, "[%s - %s] %s%s\n", timestamp(segment.Start), timestamp(segment.End), speaker, segment.Text)
		p.streamed = true
	case !p.segments && segment == nil && text != "":
		fmt.Fprint(p.w, text)
		p.streamed, p.text = true, true
	}
}

// Close ends the text with a newline, once it's printed, or writes the last
// cue of the subtitles
func (p *transcriptPrinter) Close() {
	if p.subtitles != nil {
		p.subtitles.Close()
	}
	if p.text {
		fmt.Fprintln(p.w)
		p.text = false
	}
}

// timestamp returns when in audio, as minutes and seconds, to a tenth of a
// second, such as 1:05.3, with hours when it's an hour or more
func timestamp(d time.Duration) string {
	d = d.Round(100 * time.Millisecond)
	hours, minutes := int(d/time.Hour), int(d%time.Hour/time.Minute)
	seconds := float64(d%time.Minute) / float64(time.Second)
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%04.1f", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%04.1f", minutes, seconds)
}
