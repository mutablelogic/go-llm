// Chat with a model in the terminal, keeping the conversation in memory,
// without a server, or a store of sessions:
//
//	go run ./examples/chat
//
// Each line is a message, and the model's reply is streamed. The provider
// is Ollama, when OLLAMA_ADDR is set, or OpenAI, when OPENAI_API_KEY is, or
// otherwise Eliza, as in the ask example.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func main() {
	model := flag.String("model", "", "Model, by name, or by provider and name")
	system := flag.String("system", "Be brief", "System prompt")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, *model, *system); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, model, system string) error {
	r, err := registry.New()
	if err != nil {
		return err
	}
	if model, err = addProvider(ctx, r, model); err != nil {
		return err
	}

	// The session, which isn't stored: its settings, and the conversation,
	// which each reply is added to, and which is shortened to fit the
	// model's context, as it's sent, without changing it
	session := schema.Session{SessionMeta: schema.SessionMeta{GeneratorMeta: schema.GeneratorMeta{
		Model:  types.Ptr(model),
		System: types.Ptr(system),
	}}}

	fmt.Printf("Chatting with %s. Type a message, or press Ctrl+D to end.\n\n> ", model)
	for {
		text, err := readLine(ctx)
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
			fmt.Println()
			return nil
		} else if err != nil {
			return err
		}
		if text = strings.TrimSpace(text); text == "" {
			fmt.Print("> ")
			continue
		}
		message := &providerschema.Message{MessageMeta: providerschema.MessageMeta{
			Sender:  "user",
			Content: []providerschema.ContentBlock{{Text: types.Ptr(text)}},
		}}
		response, err := r.WithSession(ctx, session, []llm.Message{message}, func(what llm.StreamType, text string) {
			if what == llm.StreamText {
				fmt.Print(text)
			}
		})
		if errors.Is(err, context.Canceled) {
			fmt.Println()
			return nil
		} else if err != nil {
			return err
		}

		// The message, and the reply, are added to the conversation
		session.Messages = append(session.Messages, response.Messages...)
		fmt.Printf("\n\n> ")
	}
}

// stdin is the input, which is read a line at a time
var stdin = bufio.NewReader(os.Stdin)

// readLine returns a line of input, without waiting for it once the context
// is done, such as when Ctrl+C is pressed, or io.EOF at the end of the input
func readLine(ctx context.Context) (string, error) {
	type line struct {
		text string
		err  error
	}
	ch := make(chan line, 1)
	go func() {
		text, err := stdin.ReadString('\n')
		if err == io.EOF && text != "" {
			err = nil
		}
		ch <- line{text, err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case l := <-ch:
		return l.text, l.err
	}
}

// addProvider adds a provider to the registry, which is Ollama, when
// OLLAMA_ADDR is set, or OpenAI, when OPENAI_API_KEY is, or Eliza, and
// returns the model, which is the one given, or the provider's default
func addProvider(ctx context.Context, r *registry.Registry, model string) (string, error) {
	var provider schema.Provider
	var fallback string
	switch {
	case os.Getenv("OLLAMA_ADDR") != "":
		provider = schema.Provider{Type: llm.Ollama, ProviderMeta: schema.ProviderMeta{URL: os.Getenv("OLLAMA_ADDR")}}
		fallback = "llama3.2"
	case os.Getenv("OPENAI_API_KEY") != "":
		provider = schema.Provider{Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{APIKey: "${OPENAI_API_KEY}"}}
		fallback = "gpt-4o-mini"
	default:
		provider = schema.Provider{Type: llm.Eliza}
		fallback = "eliza-1966-en"
	}
	if _, err := r.CreateProvider(ctx, provider); err != nil {
		return "", err
	}
	if model == "" {
		model = fallback
	}
	return model, nil
}
