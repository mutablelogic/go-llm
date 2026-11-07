# go-llm

[![Go Reference](https://pkg.go.dev/badge/github.com/mutablelogic/go-llm.svg)](https://pkg.go.dev/github.com/mutablelogic/go-llm)
[![License](https://img.shields.io/badge/license-Apache-blue.svg)](LICENSE)

go-llm is a server, command-line tool and Go SDK which puts large language models and speech models from several providers behind one interface. You add providers, such as Ollama, OpenAI, Anthropic, Google Gemini, Mistral, [go-whisper](https://github.com/mutablelogic/go-whisper) and ElevenLabs, and then ask any of their models to reply to messages, call tools, run agents, embed text and images, read the text in documents, and transcribe and translate speech. The same operations are available as a REST API, from the `llm` command, which includes an interactive chat in the terminal, or directly from Go.

## Quick Start

### 1. Run the server, with Docker

The image has the server, and its web UI, and keeps its providers, sessions, agents and memory in the `/data` volume:

> [!WARNING]
> **Don't run the server on the internet.** It has no authentication yet, so anyone who can reach it can use your providers' API keys, read your chats, memory and files, run its tools, and change its providers and agents. Publish its port on `localhost` only, as below, or, to reach it from elsewhere, put it behind a proxy which signs people in, or a VPN.

```bash
docker run -d --name llm -p 127.0.0.1:8084:8084 -v llm:/data \
  -e OLLAMA_ADDR=http://host.docker.internal:11434 \
  -e OPENAI_API_KEY \
  ghcr.io/mutablelogic/llm
```

Then open [http://localhost:8084/](http://localhost:8084/), to chat, and see the models and tools:

![The web UI: the models of the providers, and a model's details](testdata/frontend.png)

`host.docker.internal` is the machine Docker runs on, such as for an Ollama server which runs there, which Docker Desktop has, and Linux has with `--add-host=host.docker.internal:host-gateway`.

When it's first started, the server has the Eliza provider, unless `NO_ELIZA` is set, and those whose variables are set: `OLLAMA_ADDR` and `GOWHISPER_ADDR`, as their URLs, and `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `DEEPSEEK_API_KEY`, `MISTRAL_API_KEY` and `ELEVENLABS_API_KEY`, whose keys are read from the container's environment when they're used, rather than stored. `UID` and `GID` set the user it runs as, such as the owner of a directory which is mounted at `/data`.

Or run it with Docker Compose, whose [compose file](docker/compose.yaml) has every variable the server reads, from the environment, or a `.env` file, and, with the `ollama` profile, an Ollama server, which `OLLAMA_ADDR=http://ollama:11434` adds. It publishes the server's port on `localhost` only, unless `LLM_BIND` says otherwise, such as `0.0.0.0`, for a network you trust:

```bash
docker compose -f docker/compose.yaml --profile ollama up -d
```

### 2. Install the command-line tool

The command-line tool, without the server, is built with the `client` tag. Install it with Go:

```bash
go install -tags client github.com/mutablelogic/go-llm/cmd/llm@latest
```

Or, in a clone of the repository, run `make client`, which builds it into `build/llm`.

It connects to the server at `localhost:8084`, or the one at `--http.addr`, or `LLM_ADDR`, such as `LLM_ADDR=nas.local:8084`.

### 3. Add providers

Providers which weren't added when the server was first started are added with the command-line tool. An API key is given by the name of its variable, as `--api-key-credential`, which the server reads from its own environment, which is the container's, rather than the command-line tool's, so the key isn't stored, or sent:

```bash
# Providers
llm provider-create-ollama http://host.docker.internal:11434
llm provider-create-openai --api-key-credential OPENAI_API_KEY
llm provider-create-anthropic --api-key-credential ANTHROPIC_API_KEY
llm provider-create-gemini --api-key-credential GEMINI_API_KEY
llm provider-create-mistral --api-key-credential MISTRAL_API_KEY
llm provider-create-gowhisper http://host.docker.internal:8081
llm provider-create-elevenlabs --api-key-credential ELEVENLABS_API_KEY

# The providers, and their models, in order of name
llm providers
llm models

# The models which can do something, such as read images and call tools
llm models --capability image_input,tools

# The models which make images, or speak
llm models --capability image_output
llm models --capability speech_output
```

### 4. Ask, and chat

Then ask models to do something. A command's model is kept, and used until another is given:

```bash
# Ask a question, and stream the reply
llm ask --model gpt-4o-mini "Explain a KV cache in two sentences"

# Ask about files: text, such as code, is sent as text, and an image or a
# PDF as an attachment, which the model must accept
llm ask --attach main.go "What does this do?"
llm ask --model gpt-4o -a chart.png -a notes.md "Do these agree?"

# Upload a file once, as a resource, and attach it by its URI, which a chat
# links to its session; a chat uploads the files it's given to its session
llm resource-create report.pdf
llm chat -a resource:///<hash> "What does this say?"
llm ocr --model mistral-ocr-latest resource:///<hash>

# Embed text, or an image
llm embed --model mistral-embed "A cat asleep in the sun"

# The text of a document, as Markdown
llm ocr --model mistral-ocr-latest report.pdf

# Speak text, as audio, in a voice
llm speak --model ollama:phonon:latest --voice Freya -o hello.wav "Hello, world"

# The speech in audio, as text, or as subtitles, with speakers
llm transcribe --model whisper-1 talk.m4a
llm transcribe --model voxtral-mini-latest --opt speakers=true --format srt talk.m4a

# Speech in any language, in English
llm translate --model whisper-1 interview.wav

# Download a model
llm model-load ollama:llama3.2
```

Or chat in a session, which the server keeps. A session is kept, and used until another is given:

```bash
# A session, with a model, a system prompt, and tools it can call
llm session-create --model ollama:llama3.2 --system "Be brief" --tools "weatherapi:*"

# Chat in the terminal, or ask once, in the session
llm chat
llm chat "Will it rain in Berlin tomorrow?"

# Its messages, and its sessions
llm session-messages
llm sessions
```

### Without Docker

Install the server and command-line tool, which are both `llm`, with Go, and run the server, which listens on `localhost:8084`, and keeps its data in the user's cache directory:

```bash
go install github.com/mutablelogic/go-llm/cmd/llm@latest
llm run
```

A server installed with `go install` serves the API, without the web UI, which `make`, from a clone of the repository, builds, with npm, into `build/llm`.

## Architecture

```mermaid
flowchart LR
    subgraph Clients
        direction TB
        CLI["`**llm** command
        and interactive chat`"]
        Client["`**registry/httpclient**
        Go client`"]
        UI["`**frontend**
        web UI`"]
    end

    subgraph Server["Server, or your own Go program"]
        direction TB
        API["`**httphandler**
        REST API and events`"]
        Registry["`**registry**
        providers, models, sessions`"]
        Toolkit["`**toolkit**
        tools, in namespaces`"]
        Agents["`**agents**
        prompts which models call as tools`"]
        Memory["`**memory**
        what every session knows`"]
        Heartbeat["`**heartbeat**
        prompts which fire on a schedule`"]
        Events["`**events**
        heartbeats which fire, and triggers, run by the event agent, with their outcomes`"]
        Resource["`**resource**
        uploaded content, by its hash`"]
        Stores[("`**stores**
        providers, sessions, agents, memory, heartbeats, event outcomes, resources`")]
    end

    subgraph Providers
        direction TB
        Ollama["`**ollama**`"]
        OpenAI["`**openai**
        OpenAI, Mistral, DeepSeek`"]
        Anthropic["`**anthropic**`"]
        Gemini["`**gemini**`"]
        Whisper["`**gowhisper**`"]
        ElevenLabs["`**elevenlabs**`"]
        Eliza["`**eliza**
        for testing`"]
    end

    subgraph Tools
        direction TB
        HA["Home Assistant"]
        News["NewsAPI"]
        Weather["WeatherAPI"]
        Search["Ollama web search"]
    end

    CLI --> Client
    Client --> API
    UI --> API
    API --> Registry
    Registry --> Stores
    Registry --> Toolkit
    Toolkit --> Agents
    Toolkit --> Memory
    Agents --> Registry
    API --> Memory
    Memory --> Stores
    Toolkit --> Heartbeat
    API --> Heartbeat
    Heartbeat --> Stores
    Heartbeat --> Events
    API --> Events
    Events --> Agents
    Events --> Stores
    API --> Resource
    Resource --> Stores
    Toolkit --> Tools
    Registry --> Ollama & OpenAI & Anthropic & Gemini & Whisper & ElevenLabs & Eliza

    classDef highlight fill:#e8f0fe,stroke:#1a73e8,stroke-width:2px,color:#111
    class CLI,Client,UI,Registry,Toolkit,Agents,Memory,Heartbeat,Events,Resource highlight
    class Ollama,OpenAI,Anthropic,Gemini,Whisper,ElevenLabs,Eliza highlight
```

- The **client and CLI**: the `llm` command, in [cmd/llm](cmd/llm), runs the server, and is its client, with commands for each operation, and an interactive chat in the terminal, which streams replies and follows other clients of the same session. The Go client, [registry/httpclient](registry/httpclient), is what it uses to talk to the server.
- The **web UI**, in [frontend](frontend), which the server serves, when it's built with `make`, is a page to chat in sessions, with replies streamed, to see the models and tools, to add, edit and delete providers, whose API keys are given as the names of environment variables on the server, so the keys aren't stored, to load and delete models, and to create, edit and delete agents, as YAML, apart from the built-in ones, which can be copied.
- The **registry**, in [registry](registry), finds a model by name, checks what it can do, and asks its provider. It keeps providers, sessions and agents in stores, runs the agent loop, which calls tools until the model replies, and publishes events about what changed. A session which hasn't changed, such as by being chatted in, for 14 days, or `--session.retention`, such as `30d`, or `0` to keep sessions, is deleted, with its files, when the server starts, and every hour, unless it has a heartbeat, which fires again, or fired in the last week.
- The **toolkit**, in [toolkit](toolkit), holds the tools which models can call, in namespaces, such as `homeassistant` or `weatherapi`, and streams what they log, and their progress, as they run.
- The **memory**, in [memory](memory), is what every session and agent knows: the details of the person, and of the assistant, such as their names, where the person is, and their time zone and language, each known, not known, or declined, and memories, which are what a model keeps beyond the conversation, such as the person's preferences, or the people in their life, under keys. It's the toolkit's `memory` namespace, whose system prompt has the details, and how to search, keep and use memories, and whose tools remember them. In a session whose model can call them, each of the person's messages starts with when they sent it, in their time zone, or UTC when it isn't known, such as `(Saturday 2026-11-07T12:38+01:00)`, as it's sent to the model, rather than the system prompt having the time, which would change it each turn, so it couldn't be cached.
- **Heartbeats**, in [heartbeat](heartbeat), are prompts for a session, which fire once, at a time, or again and again, on a schedule, such as a reminder, or a briefing each morning. A schedule is in its time zone, or the person's, which the memory has, so it follows them. It's the toolkit's `heartbeat` namespace, whose tool schedules them, and a heartbeat which fires is run, in a session of its own, whose identifier is the heartbeat's, with the model of the session it's for.
- **Events**, in [registry](registry), are what happens when a heartbeat fires, or something else, such as a webhook, triggers one, with `POST /llm/event`. Each is run in the background by the `event` agent, in a session of its own, with the model and tools of the session it's for, and at most `--llm.max-events` at once. The agent decides what the person is told, and whether they're alerted, notified, or it's ignored. Its outcome is kept in a store, until it's delivered into the session it's for, and is listed at `/llm/event`, and in the web UI's Events panel.
- **Resources**, in [resource](resource), are content, such as images, documents and audio, which is uploaded, kept once, by its SHA-256, and referred to as `resource:///<hash>`. A resource is in a session, which it's deleted with, or on its own, when it's deleted once it hasn't been used for a day. Its manager keeps them in a store, such as files, or, later, a database.
- **Agents** are named prompts, with their model, tools and the schemas of their input and reply, which models call as tools in the `agent` namespace. The [agents](agents) folder has examples, which the server adds when it starts.
- The **providers**, in [provider](provider), each wrap a provider's API, and do what it can: generate, embed, read documents, transcribe and translate speech, speak text, or make images.

The registry can be used without the server, from your own Go program, as the [examples](examples) show.

## Features

- **Providers**: add, update and remove providers while the server runs, and keep them in a store between restarts. A model is referred to by its name, or by its provider and name, such as `ollama:llama3.2:latest`.
- **Ask**: a model's reply to messages, with a system prompt, options, tools whose calls are returned for the caller to run, and structured output with a JSON schema. Replies can be streamed as server-sent events.
- **Chat**: a conversation with a model, in a session, which the server keeps, with its model, system prompt, tools, options and schema. The model calls the tools until it replies, and the conversation is shortened to fit its context. Chat in the terminal, in the web UI, or with the API, whose replies are streamed, and every client of a session sees its new messages, and whether someone is typing, or the model is replying. A conversation can change its model, such as from Claude to Gemini, and back, as each provider is sent the others' calls of tools, and their results, in a form it accepts. An attachment is sent to a model which can take it, by its capabilities, such as `image_input` for an image, and otherwise is a note, with its URI, which the model can give to a tool, such as `llm:transcribe`.
- **Agents**: named definitions of a prompt, its input, and how a model is asked it, with its system prompt, tools, options and the schema of its reply, which are kept, in YAML or JSON, and which models call as tools, in the `agent` namespace, however deep, with their progress streamed, and their tokens counted as their caller's. The server adds example agents when it starts, such as one which translates text, and the `event` agent, which runs a heartbeat which fires, in its own session, with what the system says of the person and the time in a `<system-reminder>` block, and replies with what the person is told, and whether they're alerted, notified, or it's ignored.
- **Embed**: a vector for each message, of its text, and of images and audio for models which embed them.
- **OCR**: the text of images and documents, such as PDFs, as Markdown, page by page, with the regions of each page and their boxes.
- **Speak**: text as audio, with Ollama's `phonon`, OpenAI's and Gemini's speech models, in a voice, and as instructions say, which `llm speak`, and `POST /llm/speak`, return, and the `llm:speak` tool keeps as a file of the chat.
- **Illustrate**: images, from a prompt, or by changing, or combining, images, with Gemini's Nano Banana models, which `llm illustrate`, and `POST /llm/illustrate`, keep as files, and the `llm:illustrate` tool keeps as files of the chat, so any model can make images. A model which makes images in a conversation, such as Nano Banana, has them kept as files of the chat, and shown, and can change them, as it's asked.
- **Transcribe and translate**: the speech in audio, as text, in its language or in English, with timed segments and speakers, streamed as it's transcribed, and printed as SRT or WebVTT subtitles.
- **Models**: download, delete and unload models, for providers which store them, such as Ollama and go-whisper, and list them by what they can do, such as `image_input`, which is seeing images, `image_output`, which is making them, `speech_output` or `ocr`, which the table below describes, with `llm models --capability`, or the `llm:models` tool, so a model, or an agent, can choose one.
- **Tools**: a toolkit of tools, in namespaces, for Home Assistant, NewsAPI, WeatherAPI and Ollama's web search, which models can call.
- **Memory**: the details of the person, and of the assistant, such as their names, location, time zone and language, which every session knows, as a model learns them, or they're set, and which can be declined, so they're never asked for; and memories, which a model searches, keeps and forgets, as Markdown, under keys, with versions, so a change is made to a memory as it was last seen. The server keeps each memory in a file, which can be read, and edited, by hand.
- **Resources**: content, such as images, documents and audio, which is uploaded, or which a tool returns, as an MCP tool can, which is kept as a resource of the session the tool runs in, with a link to it in the tool's result, rather than its data, as a file, or the request's body, and streamed into the store, so a large file isn't held in memory, kept once, by its hash, however often it's uploaded, and downloaded, in ranges. A resource is in a session, and deleted with it, or on its own, and deleted once it hasn't been used for a day. Content which no resource refers to, and uploads which were interrupted, are cleaned up.
- **Heartbeats**: prompts for a session, which a model schedules, or the API, once, at a time, or on a schedule, as a cron expression, such as a reminder at six, or a briefing every weekday at eight, in a time zone, or the person's, which they follow, as it changes. When the clocks change, a time fires once, and a heartbeat which was due when the server wasn't running fires once, when it starts. One which won't fire again is deleted a week after it last fired. Servers which share a store fire a heartbeat once. A heartbeat which fires is run by the `event` agent, in a session of its own, with the model and tools of the session it's for, which replies with what the person is told, and how important it is. Its outcome is stored, as ignored, or pending, until it's delivered, which is next, and listed at `/llm/event`.
- **Events**: a stream of changes to the providers and their models, to sessions, with whether someone is typing in a session, or its model is replying, so two chat windows on one session follow each other, to the memory's details and memories, and to heartbeats, including when they fire.
- **OpenAPI**: the REST API is described by an OpenAPI specification, which `llm openapi` shows as documentation in a browser, or outputs as JSON or YAML.
- **OpenTelemetry**: traces of requests, through the registry, providers and tools.

## Providers

| Type | Provider | Generate | Embed | OCR | Transcribe | Translate | Speak | Images | Models |
|------|----------|:--------:|:-----:|:---:|:----------:|:---------:|:-----:|:------:|--------|
| `ollama` | [Ollama](https://ollama.com) | ✅ | ✅ | ✅ (images) | | | ✅ (phonon) | | Download, delete, unload |
| `openai` | [OpenAI](https://platform.openai.com) | ✅ | ✅ | | ✅ | ✅ (whisper) | ✅ (tts) | | |
| `anthropic` | [Anthropic](https://www.anthropic.com)'s Claude models, which count tokens exactly, to fit a conversation to the model's context | ✅ | | | | | | | |
| `gemini` | [Google](https://ai.google.dev)'s Gemini models, which count tokens exactly, and keep a conversation another model started | ✅ | ✅ (text, images, audio, PDFs) | | ✅ (transcribe) | | ✅ (tts) | ✅ (Nano Banana) | |
| `openai` | [Mistral](https://mistral.ai), at `https://api.mistral.ai/v1` | ✅ | ✅ | ✅ | ✅ | | | | |
| `openai` | [DeepSeek](https://www.deepseek.com), and other OpenAI-compatible servers | ✅ | | | | | | | |
| `gowhisper` | [go-whisper](https://github.com/mutablelogic/go-whisper) | | | | ✅ | ✅ | | | Download, delete |
| `elevenlabs` | [ElevenLabs](https://elevenlabs.io) | | | | ✅ | | | | |
| `eliza` | A mock of Weizenbaum's 1966 chatbot, for testing | ✅ | | | | | | | |

What a model can do depends on the model, which its provider reports, where it knows: such as whether it calls tools, thinks, sees images, or recognizes text. A request for something a model is known not to do is rejected before it's sent.

### Capabilities

What a model can do, as its provider says, which `llm models --capability`, `GET /llm/model?capability=` and the `llm:models` tool list models by. A capability a provider doesn't say a model has, or hasn't, isn't known, so the model isn't listed for it.

| Capability | Meaning |
|------------|---------|
| `chat` | Replies to messages, in a conversation, with `ask`, `chat`, agents and events |
| `tools` | Calls tools, as it replies |
| `thinking` | Thinks, or reasons, before it replies |
| `image_input` | Understands images in a conversation, so they're sent to it |
| `audio_input` | Hears audio in a conversation |
| `video_input` | Watches video in a conversation |
| `document_input` | Reads documents, such as PDFs, in a conversation, rather than recognizing their text first |
| `image_output` | Makes images, in a conversation, and with `illustrate` and the `llm:illustrate` tool |
| `speech_output` | Speaks text, as audio, with `speak` and the `llm:speak` tool |
| `embedding` | Embeds text, and media, as vectors, with `embed` |
| `ocr` | Recognizes the text in images and documents, with `ocr` and the `llm:ocr` tool |
| `transcription` | Transcribes speech, with `transcribe` and the `llm:transcribe` tool |
| `translation` | Translates speech into English, with `translate` |

In a conversation, an attachment a model is known not to take, such as audio for a model without `audio_input`, isn't sent, but is a note, with its URI, which the model can give to a tool, such as `llm:transcribe`. Text is always sent, as text.

## CLI Commands

| Group | Commands |
|-------|----------|
| Server | `run`, `openapi` |
| Providers | `providers`, `provider`, `provider-create-ollama`, `provider-create-openai`, `provider-create-anthropic`, `provider-create-gemini`, `provider-create-mistral`, `provider-create-deepseek`, `provider-create-gowhisper`, `provider-create-elevenlabs`, `provider-create-eliza`, `provider-update`, `provider-delete` |
| Models | `models`, `model`, `model-load`, `model-delete`, `model-unload` |
| Sessions | `sessions`, `session`, `session-messages`, `session-create`, `session-update`, `session-delete` |
| Inference | `ask`, `chat`, `embed`, `ocr`, `transcribe`, `translate`, `speak`, `illustrate` |
| Events | `events` |
| Memory | `details`, `detail-set`, `detail-forget`, `memories`, `memory`, `memory-remember`, `memory-forget` |
| Heartbeats | `heartbeats`, `heartbeat`, `heartbeat-create`, `heartbeat-update`, `heartbeat-delete` |
| Resources | `resources`, `resource`, `resource-create`, `resource-get`, `resource-delete` |
| Tools and agents | `tools`, `tool`, `call`, `agents`, `agent`, `agent-create`, `agent-update`, `agent-delete` |

Run `llm <command> --help` for a command's arguments and flags. Options for a model are given with `--opt`, such as `--opt temperature=0.2`, or `--opt pages=0-2` for OCR.

A few options are for the agent loop, which asks a model, runs the tools it calls, and asks it again, rather than for the model's provider:

| Option | Default | |
|--------|---------|-|
| `max_iterations` | 10, or 3 for `ask` | The most times the model is asked for its reply. |
| `max_depth` | 3 | The most agents which run in one another, as a model calls an agent, which calls another, or itself. An agent can lower it, but not raise it. |
| `tool_timeout` | `30s` | The most time a tool has to return. |

They're given to a session, or an agent, such as `llm session-update --opt max_iterations=20 --opt max_depth=2`.

## Tools

Tools are added to the server's toolkit when their credentials, or settings, are set, as environment variables, or flags:

| Namespace | Tools | Credentials |
|-----------|-------|-------------|
| `memory` | Remember the details of the person, and the assistant, and search, keep and forget memories | None, as it's always added |
| `heartbeat` | Schedule, list, update and cancel heartbeats, which are prompts which come back to the conversation, at a time, or on a schedule | None, as it's always added |
| `llm` | `models`, `speak`, `illustrate`, `ocr`, `transcribe` (which translates too) and `resource`: list the models with capabilities, such as `ocr` or `speech_output`, speak text, make and change images, read the text in an image or document, and transcribe speech, with the model the calling model gives, and list the chat's files, as a Markdown table, or read a file, by its URI, as Markdown, a part at a time, from where the last part ended, with its metadata, such as a document's title, author, date and pages, or an image's size: text, HTML, CSV, XML and code, PDFs, Word, OpenDocument, RTF and EPUB documents, spreadsheets, emails, calendars, contacts, subtitles and SVG, with [go-metadata](https://github.com/mutablelogic/go-metadata), or say what a file is, such as audio, which isn't text. The model uses the one the person chose for each capability, which it keeps in the memory `preferred_models`, with the `memory` tools, or otherwise asks them to choose one of those `models` lists, and remembers it. Audio which is spoken is kept as a resource of the session, with the `name` and `title` the model gives, and images, documents and audio are read by their URIs, `resource:///<hash>`, which the model is told for each attachment. With a `name` or `title`, the text which `ocr` reads is kept as a Markdown file, and with a `format`, `name` or `title`, the transcript is kept, as text, Markdown, or SRT or WebVTT subtitles, as a long one, of more than 8,000 characters, always is, when only its start is returned, and the model reads the rest with `resource` | None, as it's always added. Choices are remembered with the `memory` tools |
| `weatherapi` | Current weather, forecasts and alerts | `WEATHERAPI_API_KEY`, from [weatherapi.com](https://www.weatherapi.com/signup.aspx) |
| `newsapi` | News articles, headlines and sources | `NEWSAPI_API_KEY`, from [newsapi.org](https://newsapi.org/register) |
| `homeassistant` | Query, and search, and control smart home devices | `HOMEASSISTANT_URL`, `HOMEASSISTANT_API_KEY` |
| `ollama` | Web search and fetch, with Ollama's cloud service, rather than a local Ollama | `OLLAMA_API_KEY`, from [ollama.com](https://ollama.com/settings/keys) |
| `messaging` | `connect`, `list`, `send` and `disconnect`: connect a chat, such as on Telegram, to the conversation, list its chats, send them a message, and disconnect one. See [Messaging](#messaging) | `TELEGRAM_BOT_TOKEN`, from [BotFather](https://t.me/BotFather) |

A model is given tools by name, by namespace and name, such as `weatherapi:current`, or all of a namespace's, such as `weatherapi:*`, or all of them, `*`.

## Messaging

Chats on messaging services can be connected to a conversation, as channels, so you can talk to your assistant there, such as from your phone. Telegram is the first service:

1. Make a bot with [BotFather](https://t.me/BotFather), and set its token as `TELEGRAM_BOT_TOKEN` when the server starts.
2. In a chat whose model can call the `messaging` tools, ask to talk on Telegram. The assistant gives you a link, which works once, within 15 minutes. The web UI shows it as a QR code too, made in the browser.
3. Open it, or scan the QR code with your phone, and press Start. The Telegram chat is now a channel of the conversation.

What you say in the Telegram chat is sent to the conversation, and every reply in the conversation is sent to each of its channels, wherever the message it replies to came from, as is the outcome of a heartbeat which is worth telling you. A conversation can have more than one channel, such as your own chat and a group, and a chat is a channel of one conversation, so connecting it again moves it. Everyone in a group which is connected can talk to your assistant, and hear its replies, which can have what it remembers of you, so connect only groups whose people you'd tell. Messages from chats which aren't connected are ignored.

The server reads the bot's messages by long polling, so it needs no address on the internet. Only one server can read a bot's messages, so a bot's token isn't shared between servers. Replies are formatted for Telegram: bold, italic, code, links and quotes as they are, headings in bold, lists with bullets, and tables lined up in monospace. Long replies are sent as more than one message, broken between paragraphs. Text is sent and received for now; voice messages, pictures and files are to come.

## API

The server's REST API is under `/api`, and documented in [registry/httphandler/README.md](registry/httphandler/README.md), with its events in [broadcaster/httphandler/README.md](broadcaster/httphandler/README.md), the memory's in [memory/httphandler/README.md](memory/httphandler/README.md), the heartbeats' in [heartbeat/httphandler/README.md](heartbeat/httphandler/README.md), and the resources' in [resource/httphandler/README.md](resource/httphandler/README.md), and as OpenAPI, with `llm openapi`. Inference which streams, such as ask and transcribe, sends server-sent events when `text/event-stream` is accepted.

## Go

The packages can be used without the server:

- [`llm`](https://pkg.go.dev/github.com/mutablelogic/go-llm): the interfaces of providers and models, and what they do: `Generator`, `Embedder`, `Measurer`, `Recognizer`, `Transcriber`, `Speaker`, `Illustrator` and `Downloader`, and the options for them.
- [`provider/...`](provider): a client for each provider, which implements the interfaces it can.
- [`registry`](registry): the registry of providers, which finds a model by name, checks what it can do, and asks its provider, with a store, events, and HTTP handlers and a client.
- [`toolkit`](toolkit): the toolkit of tools, which a model calls.
- [`memory`](memory): the memory, which is a namespace of the toolkit, with a store, events, and HTTP handlers and a client.
- [`heartbeat`](heartbeat): the heartbeats, which are a namespace of the toolkit, with a store, events, `Run`, which fires them, and HTTP handlers and a client.
- [`messaging`](messaging): the channels, which are chats on messaging services, such as [Telegram](messaging/telegram), connected to sessions, which are a namespace of the toolkit, with a store, and `Run`, which receives their messages.
- [`broadcaster`](broadcaster): publishes events, which are changes, to the channels which watch them, with an HTTP handler which streams them as server-sent events, and a client.

The registry is meant to be composed with other modules into a whole system, rather than to be one itself. It's created with options, which plug in what a system has:

- `WithProviderStore` keeps the providers' definitions in a store, such as a database, so they're kept when the server restarts, and are shared between servers. A `ProviderStore` lists, puts and deletes providers, each with a serial, so a change to a provider which another server has changed is a conflict, and watches the store for the changes other servers make. The providers' models aren't stored, as they're listed from the providers, and cached.
- `WithSessionStore` keeps sessions, which are conversations with models, in a store, such as a database, so they're kept when the server restarts. A `SessionStore` creates, gets, lists, puts and deletes sessions, each with a UUID and a serial, which the store sets, so a change to a session which was changed since it was read is a conflict, and watches the store for the changes other servers make. Without one, the registry's session methods return `ErrServiceUnavailable`.
- `WithAgentStore` keeps agents, which are prompts with a model, tools and a schema of their input and reply, in a store, such as a database or files, so they're kept when the server restarts. An `AgentStore` creates, gets, lists, puts and deletes agents, each with its name and a serial, so a change to an agent which was changed since it was read is a conflict. The registry's agents are tools in the toolkit's `agent` namespace, which models can call. Without one, the registry's agent methods return `ErrServiceUnavailable`.
- `WithCredentialsProvider` expands an API key's placeholder, such as `${OPENAI_KEY}`, with a secret, from the environment by default, or from a secrets manager, so keys are never stored.
- `WithToolkit` gives the registry the tools which models can call.
- `WithBroadcaster` gives the registry the broadcaster which it publishes its events to, such as one which a system's other modules share, so their events are streamed together. Without one, the registry has its own.
- `WithTracer` and `WithClientOpts` trace, and configure, the requests to providers.

Its HTTP handlers are registered with a router, alongside a system's own, and its events, such as a provider being created, or a session's model replying, are watched with `Watch`. The `llm` command, in [cmd/llm](cmd/llm), is one such system: it runs the registry, with a store which keeps its providers in the command's defaults, and its sessions and agents in files, the toolkit, and a broadcaster, which streams the registry's events, in a server.

The [examples](examples) use the packages without the server, each with Ollama, when `OLLAMA_ADDR` is set, OpenAI, when `OPENAI_API_KEY` is, or Eliza, which needs neither:

| Example | |
|---------|-|
| [ask](examples/ask) | Ask a model a question, and stream its reply |
| [tools](examples/tools) | A tool of your own, in a toolkit, which the registry runs as the model calls it |
| [chat](examples/chat) | Chat in the terminal, with the conversation kept in memory |
| [agent](examples/agent) | Run the example agent which translates text, which replies with JSON |
| [ocr](examples/ocr) | Read the text in an image, or a PDF, as Markdown, with Mistral or Ollama |
| [transcribe](examples/transcribe) | Transcribe, or translate, the speech in audio, with OpenAI, Mistral, go-whisper or ElevenLabs |

```bash
go run ./examples/ask "Explain a KV cache in two sentences"
```

## Development

```bash
make vet race-test TEST_FLAGS=-short   # As CI runs the tests
make test                              # All the tests, with coverage
```

Tests which make requests to a provider are skipped with `-short`, and unless its credentials are set, such as `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `MISTRAL_API_KEY` or `ELEVENLABS_API_KEY`, or its address, such as `OLLAMA_ADDR` or `GOWHISPER_ADDR`.

### Tests with live models

The live tests check what fakes can't: that a provider's API accepts what go-llm sends, and returns what it expects. They're `Test_Live…` tests, mostly, in each provider's package. Each runs when its variable is set, and isn't with `-short`:

| Variable | Package | What's tested | Billed |
|----------|---------|---------------|:------:|
| `OPENAI_API_KEY` | `provider/openai` | Replies, with tools, embeddings, transcription, translation, and speakers, with `gpt-4o-mini`, `text-embedding-3-small`, `whisper-1`, `gpt-4o-mini-transcribe` and `gpt-4o-transcribe-diarize`, which `OPENAI_MODEL`, `OPENAI_EMBEDDING_MODEL`, `OPENAI_WHISPER_MODEL`, `OPENAI_TRANSCRIBE_MODEL` and `OPENAI_DIARIZE_MODEL` change | Yes |
| `MISTRAL_API_KEY` | `provider/openai` | Mistral's replies, embeddings, OCR and transcription | Yes |
| `ANTHROPIC_API_KEY` | `provider/anthropic` | Replies, with tools, thinking and structured output, and counting tokens, with `claude-haiku-5-5` | Yes |
| `GEMINI_API_KEY` | `provider/gemini` | Listing models and counting tokens, which are free, embeddings, with `gemini-embedding-2`, and a reply which calls a tool, with `gemini-3.5-flash-lite` | Yes, apart from listing and counting |
| `ANTHROPIC_API_KEY` and `GEMINI_API_KEY` | `provider/gemini` | `Test_LiveSwitch`: a conversation which changes its model, from Claude to Gemini, and back, each calling a tool, so each is sent the other's calls, and their results | Yes |
| `ELEVENLABS_API_KEY` | `provider/elevenlabs` | Transcription, with speakers, with `scribe_v2` | Yes |
| `OLLAMA_ADDR` | `provider/ollama` | Its version, and models, embeddings, of text and images, and OCR, with the models of a running Ollama, which must be pulled: `embeddinggemma-2` and `deepseek-ocr`, which `OLLAMA_EMBED_MODEL` and `OLLAMA_OCR_MODEL` change | No |
| `GOWHISPER_ADDR` | `provider/gowhisper` | Its models, and transcription, with a running go-whisper server, which has `ggml-medium-q5_0` | No |

Run a provider's live tests on their own, with `-count=1`, so their results aren't cached, and `-v`, to see which ran, and which were skipped:

```bash
GEMINI_API_KEY=... go test -count=1 -v -run Live ./provider/gemini/
ANTHROPIC_API_KEY=... GEMINI_API_KEY=... go test -count=1 -v -run Test_LiveSwitch ./provider/gemini/
OLLAMA_ADDR=http://localhost:11434 go test -count=1 -v ./provider/ollama/
```

The billed tests use the cheapest models, and short requests, which cost a fraction of a cent each run, but they run whenever their keys are set, and `make test` runs every one. When your shell has keys, which the server, or the command, needs, keep them from the tests with `-short`, or by unsetting them for the tests:

```bash
make test TEST_FLAGS=-short
env -u OPENAI_API_KEY -u MISTRAL_API_KEY -u ANTHROPIC_API_KEY -u GEMINI_API_KEY -u ELEVENLABS_API_KEY go test ./...
```

## Roadmap

### Sessions

- **Compaction**: a session's conversation is compacted, so it doesn't grow without end, and each request doesn't trim the same messages again.
  - **Markers**: a message in a session which says its context starts there. The messages before the last marker aren't sent to the model, but are kept, so the UI still shows them, with a divider at each marker.
  - **Summaries in the background**: a marker is added at once, and a built-in `compact` agent then writes a summary of what came before it, in the background, which is added to the marker. The summary is sent first in the context, in a `<system-reminder>` block, so the model knows what was said. Until it's written, the model has only what's after the marker.
  - **When markers are added**: automatically, once a reply is stored, when the conversation fills more than about 80% of the model's context, and when it's asked for, with `POST /llm/session/{id}/compact`, and a Compact action in the chat. An event's session has a marker before each run, so a run sees only itself, and a summary of the runs before it.
  - **Forgetting**: forgetting a session's messages could add a marker, without a summary, rather than delete them, so they're kept.
  - The trimming each request does now, which drops tools' results, and the oldest turns, stays, for what's after the marker, so a conversation still fits the model's context while its summary is written.

### Agents and tools

- **MCP**: connectors to MCP servers, whose tools are added to the toolkit, each in its own namespace, so models can call them.
- **Delivering events**: an event's outcome, such as a reminder, is put into the chat it's for, as a message, with the files its run made, such as speech, at the right time, rather than only listed in the Events panel.
- **Channels**: Telegram chats can be connected to a conversation (see [Messaging](#messaging)). Still to come:
  - voice messages, which are transcribed, and replies spoken with `llm:speak`, sent as voice;
  - pictures and files, both ways, as the session's files;
  - the channels in the web UI, beside the chat;
  - Slack.
- **Scheduled runs in the chat**: when a heartbeat fires, its prompt goes to the chat's own model, marked as a scheduled run, with a short history of earlier runs, rather than only to the `event` agent.
- **Scheduled agents**: agents which run on a schedule, as heartbeats do.
- **Running Python** (experimental): a tool which runs Python a model writes, in WebAssembly, so it's sandboxed, without network access, with a file system of the session's files, by their names, which it reads, and to which what it writes, such as a chart, or a CSV, is kept, as the session's files.

### Server

- **Authentication and authorization**, throughout: the API, the web UI, the CLI, the MCP connectors and channels:
  - **Authentication**: people, and clients, sign in, such as with OIDC, or with API tokens, so a server which is reached from the internet isn't open to anyone.
  - **Authorization**: what each can do, by their role, or a token's scopes, such as who adds providers and agents, and who only chats, which models and tools they can use, and their own sessions, memory, files and heartbeats, which others can't see.
- **A database, and more than one server**: keep providers, sessions, agents, memory, heartbeats and resources in a database, rather than files, so more than one server can share them, and each server's clients see the changes, and heartbeats which fire, of the others.

### Providers

- **LM Studio**, which runs models locally, as Ollama does, with its own API to list, download, delete, load and unload them, as well as an OpenAI-compatible API.
- **ElevenLabs' voices**: text to speech with ElevenLabs.
- **OpenAI's images**: making and changing images with OpenAI's `gpt-image` models, as Gemini's Nano Banana models do.
- **OpenAI's Responses API**, so OpenAI's newest models can think while they call tools, and the models which only it has can be used.
- **OpenAI's token limits**, which are from memory, and should be checked.

### Improvements

- **OCR**:
  - **`glm-ocr`**, with Ollama: it has a prompt, `Text Recognition:`, but hasn't been tested with the model, and can't return regions yet.
  - **PDFs with Ollama's models**, which read images, not documents: each page of a PDF is turned into an image, and recognized.
  - **Images in documents**, such as a photo, or a scanned page, in a PDF, whose text a document's text leaves out.
- **Speech**:
  - Speakers with go-whisper's diarizing model, which only knows when the speaker changes.
  - Live transcription, over a WebSocket, as it's spoken: ElevenLabs' realtime transcription, Gemini's live models, such as `gemini-3.5-transcribe-live`, and go-whisper's streaming.
  - Translation with models which hear audio in a conversation, such as Voxtral Small.
  - Subtitles whose lengths can be set.
- **Files**, which `llm:resource` reads:
  - UTF-16 text files, which are taken for binary, as they have NULs ([#192](https://github.com/mutablelogic/go-llm/issues/192)).
  - Audio's, and video's, metadata, such as their length, which needs go-media, and so cgo.
  - Parts of a document's Markdown which end at a heading, a paragraph, or a line, rather than in the middle of one ([go-metadata#20](https://github.com/mutablelogic/go-metadata/issues/20)).

## License

go-llm is licensed under the [Apache License, Version 2.0](LICENSE).
