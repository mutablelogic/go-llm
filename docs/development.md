# Development

## Building

```bash
make                 # the server, with the web UI, into build/llm
make client          # the command line only
```

The web UI is built with npm, from the `frontend` folder, with Lit and IBM's Carbon components.

## Testing

```bash
make vet race-test TEST_FLAGS=-short   # as CI runs the tests
make test                              # all tests, with coverage
```

### Live tests

Live tests check that a provider's API accepts what go-llm sends. They run when their variable is set, and `-short` isn't:

| Variable | Package | Billed |
|----------|---------|:------:|
| `OPENAI_API_KEY` | `provider/openai` | Yes |
| `MISTRAL_API_KEY` | `provider/openai` | Yes |
| `ANTHROPIC_API_KEY` | `provider/anthropic` | Yes |
| `GEMINI_API_KEY` | `provider/gemini` | Yes, apart from listing and counting |
| `ELEVENLABS_API_KEY` | `provider/elevenlabs` | Yes, apart from voices; speaking only with `ELEVENLABS_SPEAK` |
| `OLLAMA_ADDR` | `provider/ollama` | No |
| `GOWHISPER_ADDR` | `provider/gowhisper` | No |
| `LMSTUDIO_ADDR` | `provider/lmstudio` | No |

The billed tests use the cheapest models, and cost a fraction of a cent, but `make test` runs them whenever your shell has the keys. Keep them out with `-short`, or unset the keys.

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
        heartbeats and triggers, with their outcomes`"]
        Resource["`**resource**
        uploaded content, by its hash`"]
        MCP["`**mcp**
        connectors to remote MCP servers`"]
        Stores[("`**stores**
        providers, sessions, agents, memory, heartbeats, events, resources, connectors`")]
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
        Search["Ollama and DeepSeek web search"]
        Remote["Remote MCP servers"]
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
    API --> MCP
    MCP --> Toolkit
    MCP --> Stores
    MCP --> Remote
    Toolkit --> HA & News & Weather & Search
    Registry --> Ollama & OpenAI & Anthropic & Gemini & Whisper & ElevenLabs & Eliza

    classDef highlight fill:#e8f0fe,stroke:#1a73e8,stroke-width:2px,color:#111
    class CLI,Client,UI,Registry,Toolkit,Agents,Memory,Heartbeat,Events,Resource,MCP highlight
    class Ollama,OpenAI,Anthropic,Gemini,Whisper,ElevenLabs,Eliza highlight
```

- **[`registry`](https://github.com/mutablelogic/go-llm/tree/main/registry):** finds a model by name, checks what it can do, and asks its provider. It keeps providers, conversations and agents, runs the agent loop, and publishes events.
- **[`toolkit`](https://github.com/mutablelogic/go-llm/tree/main/toolkit):** the tools models call, in namespaces.
- **[`provider`](https://github.com/mutablelogic/go-llm/tree/main/provider):** a client for each provider.
- **[`memory`](https://github.com/mutablelogic/go-llm/tree/main/memory), [`heartbeat`](https://github.com/mutablelogic/go-llm/tree/main/heartbeat), [`messaging`](https://github.com/mutablelogic/go-llm/tree/main/messaging):** namespaces of the toolkit, with their stores.
- **[`resource`](https://github.com/mutablelogic/go-llm/tree/main/resource):** files, kept by their hash.
- **[`mcp`](https://github.com/mutablelogic/go-llm/tree/main/mcp):** remote MCP clients, and connectors.
- **[`cmd/llm`](https://github.com/mutablelogic/go-llm/tree/main/cmd/llm):** the server, and the command line.

## Using the packages

The Go API reference is at [pkg.go.dev/github.com/mutablelogic/go-llm](https://pkg.go.dev/github.com/mutablelogic/go-llm).

The registry is meant to be composed into a larger system. Its options plug in what that system has:

- **`WithProviderStore`, `WithSessionStore`, `WithAgentStore`:** keep providers, conversations and agents in a store, such as a database. Each item has a serial, so a stale change is a conflict, and stores can watch for other servers' changes.
- **`WithCredentialsProvider`:** expands an API key's placeholder, such as `${OPENAI_KEY}`, from the environment by default, or from a secrets manager.
- **`WithToolkit`:** the tools models can call.
- **`WithBroadcaster`:** shares an event stream with other modules.
- **`WithTracer`, `WithClientOpts`:** trace, and configure, requests to providers.

The [examples](https://github.com/mutablelogic/go-llm/tree/main/examples) use the packages directly, with Ollama (`OLLAMA_ADDR`), OpenAI (`OPENAI_API_KEY`), or Eliza:

| Example | What it does |
|---------|--------------|
| [ask](https://github.com/mutablelogic/go-llm/tree/main/examples/ask) | Ask a question, and stream the reply |
| [tools](https://github.com/mutablelogic/go-llm/tree/main/examples/tools) | Your own tool, which the model calls |
| [chat](https://github.com/mutablelogic/go-llm/tree/main/examples/chat) | Chat in the terminal |
| [agent](https://github.com/mutablelogic/go-llm/tree/main/examples/agent) | Run an agent that replies with JSON |
| [ocr](https://github.com/mutablelogic/go-llm/tree/main/examples/ocr) | Read an image or PDF as Markdown |
| [transcribe](https://github.com/mutablelogic/go-llm/tree/main/examples/transcribe) | Transcribe or translate speech |


```bash
go run ./examples/ask "Explain a KV cache in two sentences"
```

## Contributing

Please open an issue first, so a change can be discussed. See [Feedback & Issues](feedback-and-issues.md).
