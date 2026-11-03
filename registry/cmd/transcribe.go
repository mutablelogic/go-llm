package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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
}

type TranslateCmd struct {
	File   string                     `arg:"" name:"file" type:"existingfile" help:"The audio, whose speech, in any language, is translated into English."`
	Model  string                     `name:"model" help:"Model name, or provider and model name separated by a colon, such as openai:whisper-1. Defaults to the model last used for translating speech." optional:""`
	Opt    map[string]json.RawMessage `name:"opt" help:"Options, by key, such as segments=true, which can be given more than once." optional:""`
	Stream bool                       `name:"stream" negatable:"" default:"true" help:"Print the transcript as it's translated, or with --no-stream, once it's complete."`
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
	return runTranscribe(ctx, "llm.TranscribeCmd", defaultTranscribeModelKey, cmd.File, cmd.Model, cmd.Opt, cmd.Stream, (*httpclient.Client).Transcribe)
}

// Run prints the English text of the speech in the file, as TranscribeCmd
// does, and keeps the model as the default for translating speech
func (cmd *TranslateCmd) Run(ctx server.Cmd) error {
	return runTranscribe(ctx, "llm.TranslateCmd", defaultTranslateModelKey, cmd.File, cmd.Model, cmd.Opt, cmd.Stream, (*httpclient.Client).Translate)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// runTranscribe transcribes, or translates, the speech in the file, with the
// model, or the default model, by its key, which is the model once it's
// transcribed the speech, with the client's method, and prints it as it's
// transcribed, when it's streamed
func runTranscribe(ctx server.Cmd, name, key, file, model string, opt map[string]json.RawMessage, stream bool, fn func(*httpclient.Client, context.Context, schema.TranscribeRequest, llm.TranscriberStreamFn) (*schema.TranscribeResponse, error)) error {
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
	printer := &transcriptPrinter{segments: segments || speakers}
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
// when it was spoken, and who spoke it
type transcriptPrinter struct {
	segments bool // Whether segments are printed, rather than text
	streamed bool // Whether anything has been printed
	text     bool // Whether text has been printed, without a newline
}

// Stream prints a piece of the text, or a segment
func (p *transcriptPrinter) Stream(text string, segment *llm.TranscriptSegment) {
	switch {
	case p.segments && segment != nil:
		speaker := ""
		if segment.Speaker != "" {
			speaker = segment.Speaker + ": "
		}
		fmt.Printf("[%s - %s] %s%s\n", timestamp(segment.Start), timestamp(segment.End), speaker, segment.Text)
		p.streamed = true
	case !p.segments && segment == nil && text != "":
		fmt.Print(text)
		p.streamed, p.text = true, true
	}
}

// Close ends the text with a newline, once it's printed
func (p *transcriptPrinter) Close() {
	if p.text {
		fmt.Println()
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
