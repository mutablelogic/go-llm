# Features

## Using it

- **Web UI:** chat, with replies streamed and files attached; browse models and tools; add providers, connectors and agents. Every page follows changes as they happen.
- **Command line:** a command for each operation, and a chat in the terminal, which follows other clients of the same conversation.
- **REST API,** described with OpenAPI, with server-sent events for streaming.
- **Go packages,** which work on their own, without the application.

## Conversations

- **Chat** in conversations go-llm keeps, with their model, system prompt, tools and options.
- **Tool calls:** the model calls tools until it replies.
- **Memory window:** long conversations are shortened, and summarized, to fit the model's context.
- **Switching models,** such as from Claude to Gemini and back, in the same conversation.
- **Everyone sees the same conversation:** every client of a conversation sees new messages, and who's typing.
- **Dictation** in the web UI, with the browser's speech recognition.

## Models

- **Many providers,** added, changed and removed while go-llm runs.
- **By name:** a model is referred to by its name, or by its provider and name, such as `ollama:llama3.2:latest`.
- **Capabilities:** list models by what they can do, such as `speech_output` or `ocr`.
- **Local models:** download, load, unload and delete them, where the provider stores them, such as Ollama, LM Studio and go-whisper.

## What models do

- **Embed:** vectors for text and media.
- **OCR:** a document's text, as Markdown.
- **Speak** in a chosen voice, by its name, such as Matilda.
- **Illustrate:** images, with Gemini's Nano Banana models.
- **Transcribe and translate** speech, with speakers, or as subtitles.

## Assistant

- **Tools:** weather, news, Home Assistant, web search, and any remote MCP server, through connectors.
- **Agents:** prompts with their own model, tools and schemas, which models call as tools.
- **Memory:** facts about the person and the assistant, and memories with versions.
- **Tasks:** prompts which run on a schedule, such as reminders or a morning briefing.
- **Messaging:** talk to your assistant on Telegram, by text or voice.
- **Files:** uploaded, or made by tools, kept once, and read a part at a time.

## Operations

- **Events:** a stream of changes to providers, models, conversations, memory, tasks and connectors.
- **OpenTelemetry:** traces through the registry, providers and tools.
