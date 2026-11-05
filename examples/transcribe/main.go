// Transcribe the speech in audio, as text, streamed as it's transcribed, or
// translate it into English, with a registry of providers, without a
// server:
//
//	go run ./examples/transcribe talk.mp3
//	go run ./examples/transcribe -segments -speakers interview.wav
//	go run ./examples/transcribe -translate interview.wav
//
// The provider is OpenAI, when OPENAI_API_KEY is set, whose model is
// whisper-1, or Mistral, when MISTRAL_API_KEY is, whose model is
// voxtral-mini-latest, or go-whisper, when GOWHISPER_ADDR is, whose model is
// set with -model, or ElevenLabs, when ELEVENLABS_API_KEY is, whose model is
// scribe_v1. Only OpenAI's whisper-1 translates.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/provider/openai"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

func main() {
	model := flag.String("model", "", "Model which transcribes speech, by name, or by provider and name")
	language := flag.String("language", "", "Language which is spoken, such as en, or detected otherwise")
	segments := flag.Bool("segments", false, "Print the segments, with when each was spoken")
	speakers := flag.Bool("speakers", false, "Say who spoke each segment, for providers which can")
	translate := flag.Bool("translate", false, "Translate the speech into English")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: transcribe [-model name] [-language en] [-segments] [-speakers] [-translate] file")
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	opts := options{language: *language, segments: *segments || *speakers, speakers: *speakers, translate: *translate}
	if err := run(ctx, *model, flag.Arg(0), opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// options are how the speech is transcribed
type options struct {
	language           string
	segments, speakers bool
	translate          bool
}

func run(ctx context.Context, name, path string, o options) error {
	r, err := registry.New()
	if err != nil {
		return err
	}
	if name, err = addProvider(ctx, r, name); err != nil {
		return err
	}
	model, err := r.GetModel(ctx, name)
	if err != nil {
		return err
	}

	// The audio, as a resource, whose type is that of its data, or its name,
	// such as audio/mpeg
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	audio := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{
		ContentType: providerschema.ResourceType(data, filepath.Base(path)),
		Data:        data,
	}}

	// The options, which the provider rejects, when it can't do them
	var opts []llm.Opt
	if o.language != "" {
		opts = append(opts, llm.WithLanguage(o.language))
	}
	if o.segments {
		opts = append(opts, llm.WithSegments(true))
	}
	if o.speakers {
		opts = append(opts, llm.WithSpeakers(true))
	}

	// The text is streamed as it's transcribed, unless the segments are
	// printed, each once it's transcribed
	stream := func(text string, segment *llm.TranscriptSegment) {
		switch {
		case o.segments && segment != nil:
			printSegment(*segment)
		case !o.segments:
			fmt.Print(text)
		}
	}
	var transcript llm.Transcript
	var usage llm.Usage
	if o.translate {
		transcript, usage, err = r.Translate(ctx, model, audio, stream, opts...)
	} else {
		transcript, usage, err = r.Transcribe(ctx, model, audio, stream, opts...)
	}
	if err != nil {
		return err
	}
	if !o.segments {
		fmt.Println()
	}
	fmt.Fprintf(os.Stderr, "\n%s: %q, %d segments, %ds of audio, in %v\n", r.Name(model), transcript.Language, len(transcript.Segments), usage.Seconds, usage.Duration.Round(time.Millisecond))
	return nil
}

// printSegment prints a segment, with when it was spoken, and who spoke it,
// when it's known
func printSegment(segment llm.TranscriptSegment) {
	who := ""
	if segment.Speaker != "" {
		who = segment.Speaker + ": "
	}
	fmt.Printf("[%s → %s] %s%s\n", clock(segment.Start), clock(segment.End), who, segment.Text)
}

// clock returns the time, as minutes, seconds and tenths, such as 01:02.3
func clock(d time.Duration) string {
	d = d.Round(100 * time.Millisecond)
	return fmt.Sprintf("%02d:%04.1f", int(d.Minutes()), (d % time.Minute).Seconds())
}

// addProvider adds a provider to the registry, which is OpenAI, when
// OPENAI_API_KEY is set, Mistral, when MISTRAL_API_KEY is, go-whisper, when
// GOWHISPER_ADDR is, or ElevenLabs, when ELEVENLABS_API_KEY is, and returns
// the model, which is the one given, or the provider's model which
// transcribes speech
func addProvider(ctx context.Context, r *registry.Registry, model string) (string, error) {
	var provider schema.Provider
	var fallback string
	switch {
	case os.Getenv("OPENAI_API_KEY") != "":
		provider = schema.Provider{Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{APIKey: "${OPENAI_API_KEY}"}}
		fallback = "whisper-1"
	case os.Getenv("MISTRAL_API_KEY") != "":
		provider = schema.Provider{Name: "mistral", Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{URL: openai.MistralEndpoint, APIKey: "${MISTRAL_API_KEY}"}}
		fallback = "voxtral-mini-latest"
	case os.Getenv("GOWHISPER_ADDR") != "":
		provider = schema.Provider{Type: llm.GoWhisper, ProviderMeta: schema.ProviderMeta{URL: os.Getenv("GOWHISPER_ADDR")}}
		if model == "" {
			return "", errors.New("set -model to a go-whisper model, such as one of llm models gowhisper")
		}
	case os.Getenv("ELEVENLABS_API_KEY") != "":
		provider = schema.Provider{Type: llm.ElevenLabs, ProviderMeta: schema.ProviderMeta{APIKey: "${ELEVENLABS_API_KEY}"}}
		fallback = "scribe_v1"
	default:
		return "", errors.New("set OPENAI_API_KEY, MISTRAL_API_KEY, GOWHISPER_ADDR, or ELEVENLABS_API_KEY, for a provider which transcribes speech")
	}
	if _, err := r.CreateProvider(ctx, provider); err != nil {
		return "", err
	}
	if model == "" {
		model = fallback
	}
	return model, nil
}
