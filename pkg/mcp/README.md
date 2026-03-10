# pkg/mcp

Package `mcp` provides a concise API for building and consuming [Model Context Protocol](https://modelcontextprotocol.io) servers and clients, built on top of the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

| Sub-package | Purpose |
|---|---|
| `pkg/mcp/server` | Build an MCP server that exposes tools, prompts and resources |
| `pkg/mcp/client` | Connect to any MCP server as a client |
| `pkg/mcp/mock` | `MockTool` helper for use in tests |

---

## Server

### Creating a server

```go
import "github.com/mutablelogic/go-llm/pkg/mcp/server"

srv, err := server.New("my-server", "1.0.0",
    server.WithTitle("My Server"),
    server.WithInstructions("You can use this server to do X."),
    server.WithLogger(slog.Default()),
    server.WithKeepAlive(30 * time.Second),
)
```

| Option | Description |
|---|---|
| `WithTitle(title)` | Human-readable display name shown in MCP-aware UIs |
| `WithWebsiteURL(url)` | URL advertised in the server's implementation descriptor |
| `WithInstructions(text)` | System-prompt hint forwarded to clients during handshake |
| `WithLogger(logger)` | `slog.Logger` for server-level activity logging |
| `WithKeepAlive(d)` | Interval for client ping; zero disables keepalive |

### Serving over HTTP

`Handler()` returns an `http.Handler` implementing the MCP Streamable HTTP transport (spec 2025-03-26). Mount it at any path:

```go
http.Handle("/mcp", srv.Handler())
http.ListenAndServe(":8080", nil)
```

### Registering tools

Implement the `llm.Tool` interface and register it with `AddTools`. Embed `tool.DefaultTool` from `pkg/tool` to satisfy the optional `OutputSchema()` and `Meta()` methods without boilerplate:

```go
import (
    "context"
    "encoding/json"

    llm        "github.com/mutablelogic/go-llm"
    jsonschema  "github.com/google/jsonschema-go/jsonschema"
    tool        "github.com/mutablelogic/go-llm/pkg/tool"
)

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

if err := srv.AddTools(&EchoTool{}); err != nil { log.Fatal(err) }
```

Multiple tools in one call: `srv.AddTools(t1, t2, t3)`.  
Remove by name: `srv.RemoveTools("echo")`.  
Connected clients receive `notifications/tools/list_changed` automatically.

#### Return values

| Return type | MCP content produced |
|---|---|
| `string` | `TextContent` (no JSON quoting) |
| `*schema.Attachment` (`image/*`) | `ImageContent` |
| `*schema.Attachment` (`audio/*`) | `AudioContent` |
| `*schema.Attachment` (other) | `TextContent` via `TextContent()` |
| Any other value | JSON-encoded `TextContent` tagged `application/json` |

Tool panics are caught by the handler; a panicking tool returns `IsError=true` rather than crashing the session.

### Accessing the session inside a tool

Every `Run` call receives a `context.Context` with a `server.Session` injected. Retrieve it with `server.SessionFromContext`:

```go
func (t *EchoTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
    sess := server.SessionFromContext(ctx)

    // Log back to the client via MCP notifications/message
    sess.Logger().Info("processing", "tool", t.Name())

    // Send progress (only delivered if the client included a progress token)
    sess.Progress(0, 3, "starting…")
    sess.Progress(3, 3, "done")

    // Inspect the connected client
    if info := sess.ClientInfo(); info != nil {
        slog.Info("called by", "client", info.Name, "version", info.Version)
    }

    return "hello", nil
}
```

| Method | Description |
|---|---|
| `ID() string` | Unique identifier for this client session |
| `ClientInfo() *sdkmcp.Implementation` | Client name/version from the MCP handshake |
| `Capabilities() *sdkmcp.ClientCapabilities` | Capabilities advertised by the client |
| `Logger() *slog.Logger` | Sends `notifications/message` events to the client |
| `Progress(progress, total, message)` | Sends `notifications/progress` to the client |

`SessionFromContext` always returns a valid non-nil `Session`. In unit tests that call `Run` directly (without going through the MCP handler), a no-op session backed by `slog.Default()` is returned.

### Registering prompts

Prompts are registered from a `schema.AgentMeta`. The most convenient source is a markdown file with YAML front matter, parsed by `pkg/agent`:

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
  required: [text]
---
Summarize the following text into key points:

{{.text}}
```

```go
import "github.com/mutablelogic/go-llm/pkg/agent"

meta, err := agent.ReadFile("summarize.md")
if err != nil { log.Fatal(err) }
srv.AddPrompt(meta)
```

Remove prompts by name: `srv.RemovePrompts("summarize")`.  
Connected clients receive `notifications/prompts/list_changed` automatically.

---

## Client

### Creating a client

```go
import "github.com/mutablelogic/go-llm/pkg/mcp/client"

c, err := client.New(
    "https://mcp.example.com/mcp", // server URL
    "my-client",                   // client name
    "1.0.0",                       // client version
)
```

### Connecting

`Run` blocks until `ctx` is cancelled or the server closes the connection. Call it in a goroutine; all other methods are safe to call concurrently:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    if err := c.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
        log.Println("mcp client error:", err)
    }
}()
```

Methods like `ListTools` and `CallTool` return `client.ErrNotConnected` until `Run` has established the session.

The client auto-detects the transport: it tries the 2025-03-26 Streamable HTTP transport first, then falls back to the 2024-11-05 SSE transport if the server doesn't support Streamable.

### One-shot probe

`Probe` connects, reads the server's capabilities and metadata, then immediately disconnects. Use it to check a server without keeping a persistent session:

```go
state, err := c.Probe(ctx)
// state.Name, state.Version, state.Capabilities, state.ConnectedAt …
```

### Listing and calling tools

```go
// List all tools the server advertises
tools, err := c.ListTools(ctx)
for _, t := range tools {
    fmt.Println(t.Name(), "—", t.Description())
}

// Call a tool by name with JSON arguments
result, err := c.CallTool(ctx, "echo", json.RawMessage(`{"message":"hello"}`))
```

`CallTool` returns:

- a Go `error` if the tool reported `IsError=true`
- `json.RawMessage` if the result was tagged `application/json`
- `string` for plain text results
- `[]any` if the server returned multiple content blocks

Each tool returned by `ListTools` also implements `llm.Tool`, so its `Run` method invokes `CallTool` transparently:

```go
tools, _ := c.ListTools(ctx)
result, err := tools[0].Run(ctx, json.RawMessage(`{"message":"hello"}`))
```

### Server info

```go
name, version, protocol := c.ServerInfo()
```

Returns empty strings if the client is not yet connected.

### OAuth / authentication

Pass `client.WithAuth` to `New` to handle 401 responses. When the server returns `401`, the supplied function is called with the RFC 9728 `resource_metadata` discovery URL (resolved from the `WWW-Authenticate` header), then the connection is retried:

```go
c, err := client.New(serverURL, "my-client", "1.0.0",
    client.WithAuth(func(ctx context.Context, discoveryURL string) error {
        // Perform OAuth discovery at discoveryURL, obtain a token,
        // and store it using c's token mechanism.
        return performOAuth(ctx, discoveryURL)
    }),
)
```

### Logging

By default server-sent log messages and progress notifications are written to `slog.Default()`. Override with `client.OptOnLoggingMessage`:

```go
c, err := client.New(url, "my-client", "1.0.0",
    client.OptOnLoggingMessage(func(ctx context.Context, level, logger string, data any) {
        slog.Info("mcp", "level", level, "logger", logger, "data", data)
    }),
)
```

---

## Testing

`pkg/mcp/mock` provides `MockTool` for use in server and client tests:

```go
import mock "github.com/mutablelogic/go-llm/pkg/mcp/mock"

tool := &mock.MockTool{
    Name_:        "greet",
    Description_: "returns a greeting",
    Result_:      "hello world",
}

// Override with a custom function
tool.RunFn = func(ctx context.Context, input json.RawMessage) (any, error) {
    return "custom result", nil
}

// Make InputSchema() return an error
tool.InputSchemaErr_ = errors.New("schema unavailable")
```
