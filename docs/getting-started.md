# Getting Started

## Run go-llm with Docker

The image has the server and its web UI, and keeps its data in the `/data` volume.

```bash
docker volume create llm

docker run -d --name llm -p 127.0.0.1:8084:8084 -v llm:/data \
  -e OLLAMA_ADDR=http://host.docker.internal:11434 \
  -e OPENAI_API_KEY \
  ghcr.io/mutablelogic/llm
```

Then open [http://localhost:8084/](http://localhost:8084/).

- **`host.docker.internal`** is the machine Docker runs on, such as for a local Ollama. Docker Desktop has it. On Linux, add `--add-host=host.docker.internal:host-gateway`.
- **On first start,** go-llm adds a provider for each of these which is set:
  - `OLLAMA_ADDR`, `LMSTUDIO_ADDR` and `GOWHISPER_ADDR`, as URLs;
  - `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `DEEPSEEK_API_KEY`, `MISTRAL_API_KEY` and `ELEVENLABS_API_KEY`.
- **Keys** are read from the environment when they're used, and never stored.
- **Eliza,** a mock chatbot for testing, is added too, unless `NO_ELIZA` is set.
- **`UID` and `GID`** set the user it runs as, such as the owner of a mounted `/data` directory.

### Docker Compose

The [compose file](https://github.com/mutablelogic/go-llm/blob/main/docker/compose.yaml) lists every variable the server reads. Two profiles add servers whose models go-llm can use:

- **`ollama`:** an Ollama server. Set `OLLAMA_ADDR` too.
- **`whisper`:** a [go-whisper](https://github.com/mutablelogic/go-whisper) server, which transcribes speech. Set `GOWHISPER_ADDR` too.

```bash
OLLAMA_ADDR=http://ollama:11434 GOWHISPER_ADDR=http://whisper:8081 \
  docker compose -f docker/compose.yaml --profile ollama --profile whisper up -d
```

## Install the command-line tool

The web UI is all you need to chat. The `llm` command is for the terminal, and for automation, such as scripts, cron jobs and CI.

```bash
go install -tags client github.com/mutablelogic/go-llm/cmd/llm@latest
```

- **Go:** it needs Go 1.21 or later, which downloads the toolchain go.mod asks for, unless `GOTOOLCHAIN=local` is set.
- **The server:** it connects to `localhost:8084`, or the server at `--http.addr`, or `LLM_ADDR`, such as `LLM_ADDR=nas.local:8084`.

## Add providers

Add providers in the web UI, with **Add provider** on the Models page, or with the command line:

```bash
llm provider-create-ollama http://host.docker.internal:11434
llm provider-create-lmstudio http://host.docker.internal:1234/v1
llm provider-create-openai --api-key-credential OPENAI_API_KEY
llm provider-create-anthropic --api-key-credential ANTHROPIC_API_KEY
llm provider-create-gemini --api-key-credential GEMINI_API_KEY

llm providers
llm models
```

- **API keys** are given by the name of the server's environment variable, so the key itself is never stored or sent.
- **URLs** are as the server sees them, rather than the machine running `llm`.

See [Models and Providers](models-and-providers.md) for every provider.

## Ask, and chat

```bash
llm ask --model gpt-4o-mini "Explain a KV cache in two sentences"
llm ask --attach main.go "What does this do?"

llm session-create --model ollama:llama3.2 --system "Be brief" --tools "weatherapi:*"
llm chat                                        # in the terminal
llm chat "Will it rain in Berlin tomorrow?"     # once
```

Or chat in the web UI, with **Add conversation**. See [Conversations and Tasks](conversations-and-tasks.md).

## Without Docker

```bash
go install github.com/mutablelogic/go-llm/cmd/llm@latest
llm run
```

- **The server** listens on `localhost:8084`, and keeps its data in your cache directory.
- **No web UI:** installed this way, it serves only the API, under `/api`. Run `make` in a clone to build both, with npm, into `build/llm`.
