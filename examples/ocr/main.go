// Read the text in an image, or a document, such as a PDF, as Markdown,
// which is optical character recognition (OCR), with a registry of
// providers, without a server:
//
//	go run ./examples/ocr scan.png
//	go run ./examples/ocr -pages 0-1 report.pdf
//
// The provider is Mistral, when MISTRAL_API_KEY is set, whose model is
// mistral-ocr-latest, or Ollama, when OLLAMA_ADDR is, such as
// localhost:11434, whose model is deepseek-ocr, which reads images, rather
// than PDFs. The model is set with -model.
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
	model := flag.String("model", "", "Model which recognizes text, by name, or by provider and name")
	pages := flag.String("pages", "", "Pages of a document to read, from zero, such as 0,2-4, or every page")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: ocr [-model name] [-pages 0-1] file")
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, *model, *pages, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, name, pages, path string) error {
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

	// The file, as a resource, whose type is that of its data, or its name,
	// such as image/png or application/pdf
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	file := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{
		ContentType: providerschema.ResourceType(data, filepath.Base(path)),
		Data:        data,
	}}

	// Read its text, of the pages, when they're given
	var opts []llm.Opt
	if pages != "" {
		opts = append(opts, llm.WithOpt(llm.PagesKey, pages))
	}
	document, usage, err := r.Recognize(ctx, model, file, opts...)
	if err != nil {
		return err
	}

	// Each page's text, with a rule between them
	for i, page := range document.Pages {
		if i > 0 {
			fmt.Print("\n---\n\n")
		}
		fmt.Println(page.Text)
	}
	fmt.Fprintf(os.Stderr, "\n%s: %d pages, in %v\n", r.Name(model), usage.Pages, usage.Duration.Round(time.Millisecond))
	return nil
}

// addProvider adds a provider to the registry, which is Mistral, when
// MISTRAL_API_KEY is set, or Ollama, when OLLAMA_ADDR is, and returns the
// model, which is the one given, or the provider's model which reads text
func addProvider(ctx context.Context, r *registry.Registry, model string) (string, error) {
	var provider schema.Provider
	var fallback string
	switch {
	case os.Getenv("MISTRAL_API_KEY") != "":
		provider = schema.Provider{Name: "mistral", Type: llm.OpenAI, ProviderMeta: schema.ProviderMeta{URL: openai.MistralEndpoint, APIKey: "${MISTRAL_API_KEY}"}}
		fallback = "mistral-ocr-latest"
	case os.Getenv("OLLAMA_ADDR") != "":
		provider = schema.Provider{Type: llm.Ollama, ProviderMeta: schema.ProviderMeta{URL: os.Getenv("OLLAMA_ADDR")}}
		fallback = "deepseek-ocr"
	default:
		return "", errors.New("set MISTRAL_API_KEY, or OLLAMA_ADDR, for a provider which reads text")
	}
	if _, err := r.CreateProvider(ctx, provider); err != nil {
		return "", err
	}
	if model == "" {
		model = fallback
	}
	return model, nil
}
