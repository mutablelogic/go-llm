# go-llm

[![Go Reference](https://pkg.go.dev/badge/github.com/mutablelogic/go-llm.svg)](https://pkg.go.dev/github.com/mutablelogic/go-llm)
[![License](https://img.shields.io/badge/license-Apache-blue.svg)](LICENSE)

A server, command-line tool and Go SDK for large language models and speech, from several providers at once.

go-llm keeps a registry of providers, such as Ollama, OpenAI, Mistral, [go-whisper](https://github.com/mutablelogic/go-whisper) and ElevenLabs, and their models, and asks those models to reply to messages, call tools, embed text and images, recognize the text in documents, and transcribe and translate speech. The same operations are available as a REST API, from the `llm` command, or from Go.

## Features

- **Providers**: add, update and remove providers while the server runs, and keep them in a store between restarts. A model is referred to by its name, or by its provider and name, such as `ollama:llama3.2:latest`.
- **Ask**: a model's reply to messages, with a system prompt, options, tools whose calls are returned for the caller to run, and structured output with a JSON schema. Replies can be streamed as server-sent events.
- **Embed**: a vector for each message, of its text, and of images and audio for models which embed them.
- **OCR**: the text of images and documents, such as PDFs, as Markdown, page by page, with the regions of each page and their boxes.
- **Transcribe and translate**: the speech in audio, as text, in its language or in English, with timed segments and speakers, streamed as it's transcribed, and printed as SRT or WebVTT subtitles.
- **Models**: download, delete and unload models, for providers which store them, such as Ollama and go-whisper.
- **Tools**: a toolkit of tools, in namespaces, for Home Assistant, NewsAPI, WeatherAPI and Ollama's web search, which models can call.
- **Events**: a stream of changes to the providers and their models.
- **OpenTelemetry**: traces of requests, through the registry, providers and tools.

## Providers

| Type | Provider | Generate | Embed | OCR | Transcribe | Translate | Models |
|------|----------|:--------:|:-----:|:---:|:----------:|:---------:|--------|
| `ollama` | [Ollama](https://ollama.com) | ✅ | ✅ | ✅ (images) | | | Download, delete, unload |
| `openai` | [OpenAI](https://platform.openai.com) | ✅ | ✅ | | ✅ | ✅ (whisper) | |
| `openai` | [Mistral](https://mistral.ai), at `https://api.mistral.ai/v1` | ✅ | ✅ | ✅ | ✅ | | |
| `openai` | [DeepSeek](https://www.deepseek.com), and other OpenAI-compatible servers | ✅ | | | | | |
| `gowhisper` | [go-whisper](https://github.com/mutablelogic/go-whisper) | | | | ✅ | ✅ | Download, delete |
| `elevenlabs` | [ElevenLabs](https://elevenlabs.io) | | | | ✅ | | |
| `eliza` | A mock of Weizenbaum's 1966 chatbot, for testing | ✅ | | | | | |

What a model can do depends on the model, which its provider reports, where it knows: such as whether it calls tools, thinks, sees images, or recognizes text. A request for something a model is known not to do is rejected before it's sent.

## Quick Start

Install the server and command-line tool, which are both `llm`, with Go, or the command-line tool alone, without the server, with the `client` tag:

```bash
go install github.com/mutablelogic/go-llm/cmd/llm@latest
go install -tags client github.com/mutablelogic/go-llm/cmd/llm@latest
```

Or build them from a clone of the repository, with `make`, into `build/llm`, or the command-line tool alone with `make client`. Then run the server, which listens on `localhost:8084` by default:

```bash
llm run
```

In another terminal, add providers, with API keys from environment variables, which the server reads:

```bash
# Providers
llm provider-create-ollama localhost
llm provider-create-openai --api-key-credential OPENAI_API_KEY
llm provider-create-mistral --api-key-credential MISTRAL_API_KEY
llm provider-create-gowhisper localhost:8081
llm provider-create-elevenlabs --api-key-credential ELEVENLABS_API_KEY

# Their models, in order of name
llm models
```

Then ask models to do something. A command's model is kept, and used until another is given:

```bash
# Ask a question, and stream the reply
llm ask --model gpt-4o-mini "Explain a KV cache in two sentences"

# Embed text, or an image
llm embed --model mistral-embed "A cat asleep in the sun"

# The text of a document, as Markdown
llm ocr --model mistral-ocr-latest report.pdf

# The speech in audio, as text, or as subtitles, with speakers
llm transcribe --model whisper-1 talk.m4a
llm transcribe --model voxtral-mini-latest --opt speakers=true --format srt talk.m4a

# Speech in any language, in English
llm translate --model whisper-1 interview.wav

# Download a model
llm model-load ollama:llama3.2
```

The command-line tool connects to `localhost:8084`, or the server at `--http.addr` or `LLM_ADDR`.

## Commands

| Group | Commands |
|-------|----------|
| Server | `run`, `openapi` |
| Providers | `providers`, `provider`, `provider-create-ollama`, `provider-create-openai`, `provider-create-mistral`, `provider-create-deepseek`, `provider-create-gowhisper`, `provider-create-elevenlabs`, `provider-create-eliza`, `provider-update`, `provider-delete` |
| Models | `models`, `model`, `model-load`, `model-delete`, `model-unload` |
| Inference | `ask`, `embed`, `ocr`, `transcribe`, `translate` |
| Events | `events` |
| Tools | `tools`, `tool`, `call` |

Run `llm <command> --help` for a command's arguments and flags. Options for a model are given with `--opt`, such as `--opt temperature=0.2`, or `--opt pages=0-2` for OCR.

## Tools

Tools are added to the server's toolkit when their credentials are set, as environment variables:

| Namespace | Tools | Credentials |
|-----------|-------|-------------|
| `weatherapi` | Current weather, forecasts and alerts | `WEATHERAPI_API_KEY` |
| `newsapi` | News articles, headlines and sources | `NEWSAPI_API_KEY` |
| `homeassistant` | Query and control smart home devices | `HOMEASSISTANT_URL`, `HOMEASSISTANT_API_KEY` |
| `ollama` | Web search and fetch | `OLLAMA_API_KEY` |

A model is given tools by name, by namespace and name, such as `weatherapi:current`, or all of a namespace's, such as `weatherapi:*`, or all of them, `*`.

## API

The server's REST API is under `/api`, and documented in [registry/httphandler/README.md](registry/httphandler/README.md), and as OpenAPI, with `llm openapi`. Inference which streams, such as ask and transcribe, sends server-sent events when `text/event-stream` is accepted.

## Go

The packages can be used without the server:

- [`llm`](https://pkg.go.dev/github.com/mutablelogic/go-llm): the interfaces of providers and models, and what they do: `Generator`, `Embedder`, `Recognizer`, `Transcriber` and `Downloader`, and the options for them.
- [`provider/...`](provider): a client for each provider, which implements the interfaces it can.
- [`registry`](registry): the registry of providers, which finds a model by name, checks what it can do, and asks its provider, with a store, events, and HTTP handlers and a client.
- [`toolkit`](toolkit): the toolkit of tools, which a model calls.

The registry is meant to be composed with other modules into a whole system, rather than to be one itself. It's created with options, which plug in what a system has:

- `WithStore` keeps the providers' definitions in a store, such as a database, so they're kept when the server restarts, and are shared between servers. A `Store` lists, puts and deletes providers, each with a serial, so a change to a provider which another server has changed is a conflict, and watches the store for the changes other servers make. The providers' models aren't stored, as they're listed from the providers, and cached.
- `WithCredentialsProvider` expands an API key's placeholder, such as `${OPENAI_KEY}`, with a secret, from the environment by default, or from a secrets manager, so keys are never stored.
- `WithToolkit` gives the registry the tools which models can call.
- `WithTracer` and `WithClientOpts` trace, and configure, the requests to providers.

Its HTTP handlers are registered with a router, alongside a system's own, and its events, such as a provider being created, are watched with `Watch`. The `llm` command, in [cmd/llm](cmd/llm), is one such system: it runs the registry, with a store which keeps its providers in the command's defaults, and the toolkit, in a server.

For example, a registry with Ollama and OpenAI, which asks a model a question, streaming its reply, and transcribes the speech in a file:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// A registry, which a system would give its store of providers, with
	// registry.WithStore, and its secrets, with
	// registry.WithCredentialsProvider
	r, err := registry.New()
	if err != nil {
		panic(err)
	}

	// Providers, with API keys which are placeholders, expanded from the
	// environment
	if _, err := r.CreateProvider(ctx, schema.Provider{Name: "home", Type: llm.Ollama, URL: "localhost"}); err != nil {
		panic(err)
	}
	if _, err := r.CreateProvider(ctx, schema.Provider{Type: llm.OpenAI, APIKey: "${OPENAI_API_KEY}"}); err != nil {
		panic(err)
	}

	// Keep the providers, and their models, current, and in sync with the
	// store, until the context is cancelled
	go r.Run(ctx, slog.Default())

	// Ask a model, by its name, or its provider and name, and stream its reply
	response, err := r.Ask(ctx, schema.AskRequest{
		Model:  "openai:gpt-4o-mini",
		System: "Be brief",
		Messages: []providerschema.MessageMeta{
			{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr("Explain a KV cache")}}},
		},
	}, func(_ llm.StreamType, text string) {
		fmt.Print(text)
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n%d tokens in, %d out\n", response.Usage.InputTokens, response.Usage.OutputTokens)

	// Or find a model, and transcribe the speech in a file with it
	model, err := r.GetModel(ctx, "whisper-1")
	if err != nil {
		panic(err)
	}
	data, err := os.ReadFile("talk.m4a")
	if err != nil {
		panic(err)
	}
	audio := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{
		ContentType: providerschema.ResourceType(data, "talk.m4a"),
		Data:        data,
	}}
	transcript, _, err := r.Transcribe(ctx, model, audio, nil, llm.WithSegments(true))
	if err != nil {
		panic(err)
	}
	for _, segment := range transcript.Segments {
		fmt.Println(segment.Start, segment.Text)
	}
}
```

## Development

```bash
make vet race-test TEST_FLAGS=-short   # As CI runs the tests
make test                              # All the tests, with coverage
```

Tests which make requests to a provider are skipped with `-short`, and unless its credentials are set, such as `OPENAI_API_KEY`, `MISTRAL_API_KEY` or `ELEVENLABS_API_KEY`, or its address, such as `OLLAMA_ADDR` or `GOWHISPER_ADDR`.

## Roadmap

### Agents and tools

- **Agent loop**: ask runs the tools a model calls, on the server, and sends their results back to the model, until it replies, or a limit of turns, streaming the calls and their results, rather than returning the calls for the caller to run.
- **MCP**: connectors to MCP servers, whose tools are added to the toolkit, each in its own namespace, so models can call them.
- **Agents**: named definitions of a model, a system prompt, tools, a schema and options, which are run by name, with an input.
- **Sessions, memory and heartbeats**: conversations which are kept, so they continue across requests; memory, which a model reads and writes with tools; and agents which run on a schedule. These need a store, such as PostgreSQL.

### Providers

- **Anthropic and Google Gemini**, which the earlier version of go-llm had, for generation, and Gemini's embeddings.
- **Text to speech and images**, such as ElevenLabs' and OpenAI's voices, and image generation.
- **OpenAI's Responses API**, so OpenAI's newest models can think while they call tools, and the models which only it has can be used.
- **OpenAI's token limits**, which are from memory, and should be checked.

### Improvements

- **Attachments by URL**: a message's image or document by its URL, rather than its data, which the server doesn't yet read, as it shouldn't fetch any URL it's given.
- **OCR**: the images in documents, PDFs with Ollama's models, and `glm-ocr`'s prompt, which isn't tested.
- **Speech**:
  - Speakers with go-whisper's diarizing model, which only knows when the speaker changes.
  - ElevenLabs' realtime transcription, over a WebSocket.
  - Translation with models which hear audio in a conversation, such as Voxtral Small.
  - Subtitles whose lengths can be set.

## License

go-llm is licensed under the [Apache License, Version 2.0](LICENSE).
