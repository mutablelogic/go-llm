# pkg/mcp/server

Package `server` wraps the [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) to provide a concise API for building MCP servers that expose tools and prompts over the Streamable HTTP transport.

> **Note:** MCP Resources are not currently supported.

## Creating a server

```go
import "github.com/mutablelogic/go-llm/pkg/mcp/server"

srv, err := server.New("my-server", "1.0.0",
    server.WithTitle("My Server"),
    server.WithInstructions("You can use this server to do X."),
    server.WithLogger(slog.Default()),
    server.WithKeepAlive(30 * time.Second),
)
```

### Options

| Option | Description |
|---|---|
| `WithTitle(title)` | Human-readable display name shown in MCP-aware UIs |
| `WithWebsiteURL(url)` | URL advertised in the server's implementation descriptor |
| `WithInstructions(text)` | System-prompt hint forwarded to clients during handshake |
| `WithLogger(logger)` | `slog.Logger` for server-level activity logging |
| `WithKeepAlive(d)` | Interval for client ping; zero disables keepalive |

## Serving over HTTP

`Handler()` returns an `http.Handler` implementing the MCP Streamable HTTP transport (spec 2025-03-26). Mount it at any path:

```go
http.Handle("/mcp", srv.Handler())
http.ListenAndServe(":8080", nil)
```

## Registering tools

Implement the `tool.Tool` interface (from `pkg/tool`) and register it with `AddTools`:

```go
import (
    jsonschema "github.com/google/jsonschema-go/jsonschema"
)

// echoArgs defines the tool's input parameters. Field names and types are
// reflected into a JSON Schema automatically via jsonschema.For.
type echoArgs struct {
    Message string `json:"message" description:"The text to echo back"`
}

type EchoTool struct{ tool.DefaultTool }

func (t *EchoTool) Name() string        { return "echo" }
func (t *EchoTool) Description() string { return "Echoes the input back" }

func (t *EchoTool) InputSchema() (*jsonschema.Schema, error) {
    return jsonschema.For[echoArgs](nil)
}

func (t *EchoTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
    var args echoArgs
    json.Unmarshal(input, &args)
    return args.Message, nil
}

if err := srv.AddTools(&EchoTool{}); err != nil {
    log.Fatal(err)
}
```

Embed `tool.DefaultTool` to satisfy the optional `OutputSchema()` and `Meta()` methods without boilerplate.

Multiple tools can be registered in one call: `srv.AddTools(t1, t2, t3)`.

To remove tools by name: `srv.RemoveTools("echo", "other-tool")`.

All connected clients automatically receive a `notifications/tools/list_changed` notification whenever tools are added or removed, so they can refresh their tool list without polling.

### Return values

`Run` can return any JSON-serialisable value. The server converts it to MCP content automatically:

- `*schema.Attachment` with MIME type `image/*` → `ImageContent`
- `*schema.Attachment` with MIME type `audio/*` → `AudioContent`
- Any other value → JSON-encoded `TextContent`

## Accessing the session inside a tool

Every `Run` call receives a `context.Context` with a `server.Session` injected. Retrieve it with `server.SessionFromContext`:

```go
func (t *EchoTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
    sess := server.SessionFromContext(ctx)

    // Log a message back to the client (MCP notifications/message)
    sess.Logger().Info("processing request", "tool", t.Name())

    // Send progress updates (only delivered if the client included a progress token)
    sess.Progress(0, 3, "starting...")
    // ... do work ...
    sess.Progress(3, 3, "done")

    // Inspect the connected client
    if info := sess.ClientInfo(); info != nil {
        slog.Info("called by", "client", info.Name, "version", info.Version)
    }

    // Check client capabilities before using advanced features
    if caps := sess.Capabilities(); caps != nil && caps.Roots != nil {
        // client supports roots
    }

    return "hello", nil
}
```

### Session interface

| Method | Description |
|---|---|
| `ID() string` | Unique identifier for this client session |
| `ClientInfo() *sdkmcp.Implementation` | Client name and version from the MCP handshake |
| `Capabilities() *sdkmcp.ClientCapabilities` | Capabilities advertised by the client |
| `Logger() *slog.Logger` | `slog.Logger` that sends `notifications/message` to the client |
| `Progress(progress, total, message)` | Sends a `notifications/progress` event to the client |

`Logger` respects the log level set by the client via `SetLoggingLevel`; messages below that threshold are dropped. `Progress` is a no-op if the client did not include a progress token in the request.

`SessionFromContext` always returns a valid, non-nil `Session`. In tests that call `Run` directly (without going through the MCP handler), a no-op session backed by `slog.Default()` is returned.

## Registering prompts

Prompts are registered from a `schema.AgentMeta` value. The most convenient source is a **markdown file with YAML front matter**, parsed by the `pkg/agent` package.

A markdown file looks like this (`summarize.md`):

```markdown
---
name: summarize
title: Summarize text into key points
description: Summarizes long-form text into a structured response.
input:
  type: object
  properties:
    text:
      type: string
      description: The text to summarize
  required:
    - text
---
Summarize the following text into key points:

{{.text}}
```

The YAML front matter defines the name, title, description, and input schema. The markdown body is a Go template that becomes the rendered prompt message, with input arguments available as `{{.field}}`.

Load and register the prompt:

```go
import "github.com/mutablelogic/go-llm/pkg/agent"

meta, err := agent.ReadFile("summarize.md")
if err != nil {
    log.Fatal(err)
}
srv.AddPrompt(meta)
```

Multiple prompts can be loaded from a directory and registered in a loop. Connected clients automatically receive a `notifications/prompts/list_changed` notification when prompts are added or removed.

To remove prompts: `srv.RemovePrompts("summarize")`.
