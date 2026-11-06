package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	term "golang.org/x/term"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SpeakCmd asks a model to speak text, and writes the audio to a file, or
// standard output
type SpeakCmd struct {
	Text   []string                   `arg:"" name:"text" optional:"" help:"Text to speak, which can be more than one argument, or is read from standard input."`
	Model  string                     `name:"model" help:"Model name, or provider and model name separated by a colon, such as ollama:phonon:latest. Defaults to the model last used for speaking text." optional:""`
	Voice  string                     `name:"voice" help:"Voice to speak in, by its name, such as Freya, or alloy." optional:""`
	Opt    map[string]json.RawMessage `name:"opt" help:"Options, by key, such as audio_format=wav, speed=1.2, or instructions=\"Speak calmly\", which can be given more than once." optional:""`
	Output string                     `name:"output" short:"o" type:"path" help:"File to write the audio to, or standard output, when it's omitted, or -, which mustn't be a terminal." optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultSpeakModelKey is the key of the default model for speaking text,
// which is the model last used for it
const defaultSpeakModelKey = "llm.speak_model"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run writes the text, spoken by the model, as audio, to the file, or
// standard output, and says what was written, when it's a file. A file
// which isn't written whole is removed.
func (cmd *SpeakCmd) Run(ctx server.Cmd) (err error) {
	model := strings.TrimSpace(cmd.Model)
	if model == "" {
		model = strings.TrimSpace(ctx.GetString(defaultSpeakModelKey))
	}
	if model == "" {
		return fmt.Errorf("missing model: set one with --model, and it's used until another is set")
	}

	// The text, from the arguments, or standard input
	text := strings.TrimSpace(strings.Join(cmd.Text, " "))
	if text == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		text = strings.TrimSpace(string(data))
	}
	if text == "" {
		return fmt.Errorf("missing text")
	}
	opt := quoteOpts(cmd.Opt)
	if voice := strings.TrimSpace(cmd.Voice); voice != "" {
		if opt == nil {
			opt = map[string]json.RawMessage{}
		}
		opt["voice"], _ = json.Marshal(voice)
	}

	// The audio is written to the file, or standard output, which isn't a
	// terminal, as it's binary
	var w io.Writer = os.Stdout
	toFile := cmd.Output != "" && cmd.Output != "-"
	if toFile {
		var f *os.File
		if f, err = os.Create(cmd.Output); err != nil {
			return err
		}
		defer func() {
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				_ = os.Remove(cmd.Output)
			}
		}()
		w = f
	} else if term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("the audio isn't written to a terminal: write it to a file, with --output, or redirect it")
	}

	return withClient(ctx, "llm.SpeakCmd", func(reqctx context.Context, client *httpclient.Client) error {
		contentType, meta, err := client.Speak(reqctx, schema.SpeakRequest{Model: model, Text: text, Opt: opt}, w)
		if err != nil {
			return err
		}
		if meta.Model != "" {
			if err := ctx.Set(defaultSpeakModelKey, meta.Model); err != nil {
				return err
			}
		}
		if toFile {
			fmt.Printf("%s: %s, by %s\n", cmd.Output, contentType, meta.Model)
		}
		return nil
	})
}
