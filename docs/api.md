# API

go-llm's REST API is under `/api`, and is described with OpenAPI:

```bash
llm openapi
```

- **Streaming:** operations such as ask, chat and transcribe send server-sent events, when `text/event-stream` is accepted.
- **Changes:** `GET /api/changes` streams changes to providers, models, conversations, memory, tasks, connectors and workspaces, which the web UI follows. See [Changes](changes.md).
- **Errors** are JSON, with a `reason`, and say whether to try again, when a provider is busy, or refuses.

## By part

Each part is documented beside its handlers:

- [Registry](https://github.com/mutablelogic/go-llm/blob/main/registry/httphandler/README.md): providers, models, voices, conversations, agents, tasks' outcomes, and inference, such as ask, chat, embed, OCR, speak and transcribe.
- [Changes](changes.md), from [go-server's broadcaster](https://github.com/mutablelogic/go-server/blob/main/broadcaster/httphandler/README.md).
- [Memory](https://github.com/mutablelogic/go-llm/blob/main/memory/httphandler/README.md).
- [Tasks](https://github.com/mutablelogic/go-llm/blob/main/heartbeat/httphandler/README.md), as heartbeats.
- [Resources](https://github.com/mutablelogic/go-llm/blob/main/resource/httphandler/README.md).
- [Connectors](https://github.com/mutablelogic/go-llm/blob/main/mcp/registry/httphandler/README.md).
- [Workspaces](https://github.com/mutablelogic/go-llm/blob/main/workspace/httphandler/README.md): workspaces, and their folders and resources.

## Go

The Go API reference is at [pkg.go.dev/github.com/mutablelogic/go-llm](https://pkg.go.dev/github.com/mutablelogic/go-llm). The packages work on their own, without the application. See [Development](development.md).

- [`llm`](https://pkg.go.dev/github.com/mutablelogic/go-llm): the interfaces of providers and models, and their options.
- [`registry/httpclient`](https://pkg.go.dev/github.com/mutablelogic/go-llm/registry/httpclient): a Go client of the API, which the `llm` command uses.
