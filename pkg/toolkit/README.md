
# Tools, Prompts and Resources

Package `toolkit` provides the `Toolkit` type — an aggregator that collects tools, prompts and resources from multiple sources and presents them as a unified, queryable surface for LLMs. Sources include locally implemented builtins, remote MCP servers, and a persistent user namespace backed by the manager. At generation time, the toolkit is passed to the model so it can discover and invoke capabilities without needing to know where they came from.

The three kinds of items the toolkit manages are:

* **Tools** are callable functions with JSON input. The outputs are generated
 through running the tool's `Run` method.
* **Prompts** (otherwise known as "Agents") are reusable prompt templates, also with JSON input. In order to generate outputs from prompts, they are run through an LLM agent loop with a model.
* **Resources** are opaque blobs of data returned by tools that can be stored and retrieved by reference in subsequent tool calls.

All three of these entities output a `Resource`, which can be text, JSON, audio, video and so forth.

A toolkit holds three kinds of tools:

* **Builtins** — locally implemented tools, agents and resources registered with `AddTool`, `AddPrompt`, or `AddResource`.
* **Connector Tools, Prompts and Resources** — tools exposed by a remote MCP server, registered with `AddConnector`. Connectors are managed in the background, with automatic reconnection and updates.
* **User Prompts and Resources** — prompts and resources stored persistently by the manager (e.g. in a database), served from the reserved `"user"` namespace via the handler's `List` method.

## Toolkits and MCP

To create a toolkit, use `toolkit.New` with any number of options:

```go
type ToolkitHandler interface {
    // OnStateChange is called when a connector connects or reconnects.
    OnStateChange(llm.Connector, schema.ConnectorState)

    // OnToolListChanged is called when a connector's tool list changes.
    OnToolListChanged(llm.Connector)

    // OnPromptListChanged is called when a connector's prompt list changes.
    OnPromptListChanged(llm.Connector)

    // OnResourceListChanged is called when a connector's resource list changes.
    OnResourceListChanged(llm.Connector)

    // OnResourceUpdated is called when a specific resource (identified by uri) is updated.
    OnResourceUpdated(llm.Connector, string)

    // Call executes a prompt via the manager, passing optional input resources.
    Call(context.Context, llm.Prompt, ...llm.Resource) (llm.Resource, error)

    // List is called to enumerate items in the "user" namespace — prompts and resources
    // stored persistently by the manager (e.g. in a database). Tools are never returned
    // here because they are compiled code, not data.
    List(context.Context, ListRequest) (*ListResponse, error)

    // CreateConnector is called to create a new connector for the given URL.
    // It is called once on AddConnector, and again on each reconnect, so it must return
    // a fresh instance each time (allowing auth tokens to be refreshed).
    CreateConnector(string) (llm.Connector, error)
}

func main() {
    // Create a toolkit with builtins and a handler for connector events and prompt execution.
    tk, err := toolkit.New(
        toolkit.WithTool(myTool1, myTool2),
        toolkit.WithHandler(myHandler),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Add a remote MCP connector — namespace inferred from the server.
    // Can be called before or while Run is active.
    if err = tk.AddConnector("http://mcp-server/sse"); err != nil {
        log.Fatal(err)
    }

    // Or provide an explicit namespace.
    if err = tk.AddConnectorNS("my-server", "http://mcp-server/sse"); err != nil {
        log.Fatal(err)
    }

    // Run starts all connectors and blocks until ctx is cancelled.
    // It closes the toolkit and waits for all connectors to finish on return.
    // Connectors can be added and removed while Run is active.
    if err = tk.Run(ctx); err != nil {
        log.Fatal(err)
    }
}
```

The connector passed to each callback is the originating `llm.Connector` instance. The list-changed callbacks are notifications only — the handler calls `c.ListTools`, `c.ListPrompts`, or `c.ListResources` directly if it needs the updated contents.

## Lookup

`tk.Lookup` finds a tool, prompt, or resource by name or URI, returning `nil` if nothing matches:

```go
item := tk.Lookup(ctx, "summarize")                    // by name
item  = tk.Lookup(ctx, "my-server.summarize")          // by connector namespace.name
item  = tk.Lookup(ctx, "builtin.summarize")            // scoped to builtins
item  = tk.Lookup(ctx, "user.summarize")               // scoped to user namespace
item  = tk.Lookup(ctx, "file:///data/report")          // by URI (resources)
item  = tk.Lookup(ctx, "file:///data/report#my-server") // by URI#namespace
```

The lookup order is:

1. **`<namespace>.<name>`** — exact match scoped to a namespace. Use a connector name, `"builtin"` for locally registered items, or `"user"` for manager-backed items.
2. **`<uri>#<namespace>`** — exact URI scoped to a namespace (same values as above).
3. **`<name>`** — unscoped name, searching builtins first, then connectors in registration order, then the `"user"` namespace.
4. **`<uri>`** — unscoped URI, searching builtins first, then connectors in registration order, then the `"user"` namespace.

The return type is `any`; use a type switch to distinguish:

```go
switch v := tk.Lookup(ctx, "summarize").(type) {
case llm.Tool:
    result, err := tk.Call(ctx, v, toolkit.JSONResource(input, ""))
case llm.Prompt:
    result, err := tk.Call(ctx, v, toolkit.JSONResource(vars, ""))
case llm.Resource:
    data, err := v.Read(ctx)
}
```

## List

`tk.List` returns tools, prompts, and resources in a single call, controlled by a `ListRequest`:

```go
type ListRequest struct {
    // Namespace restricts results to a single source.
    // Use "builtin", "user", or a connector name. Empty string returns all.
    Namespace string

    // Type filters — set to true to include that type in results.
    // When all three are false (zero value), all types are returned.
    Tools     bool
    Prompts   bool
    Resources bool

    // Pagination.
    Limit  *uint // nil means no limit
    Offset uint
}

type ListResponse struct {
    Tools     []llm.Tool
    Prompts   []llm.Prompt
    Resources []llm.Resource

    // Pagination metadata.
    Count  uint // total items matched (before pagination)
    Offset uint
    Limit  uint
}
```

Examples:

```go
// Everything — tools, prompts and resources from all namespaces (zero value).
resp, err := tk.List(ctx, toolkit.ListRequest{})
if err != nil {
    log.Fatal(err)
}

// Tools only from one connector.
resp, err = tk.List(ctx, toolkit.ListRequest{
    Tools:     true,
    Namespace: "my-server",
})
if err != nil {
    log.Fatal(err)
}

// Paginate through all resources.
resp, err = tk.List(ctx, toolkit.ListRequest{Resources: true, Limit: types.Ptr(uint(10)), Offset: 20})
if err != nil {
    log.Fatal(err)
}
```

An empty `Namespace` (zero value) returns items from all sources combined. Set it to `"builtin"` for locally registered items only, `"user"` for manager-backed items only, or a connector name to scope to a single connector.

The reserved namespace `"user"` is backed by the handler's `List` method — prompts and resources stored persistently by the manager (e.g. in a database). Tools are always compiled code and are never served from the `"user"` namespace.

## Prompts

Prompts (also called agents) are reusable LLM agent definitions stored as markdown files with YAML front matter. The body is a [Go template](https://pkg.go.dev/text/template) that constructs the user message sent to the model; variables come from the `input` schema.

```markdown
---
name: summarize
title: Summarize text into key points
description: Summarizes long-form text into key points and sentiment.
model: claude-haiku-4-5-20251001
provider: anthropic
system_prompt: |
  You are an expert summarizer.
input:
  type: string
output:
  type: object
  properties:
    summary:
      type: string
    key_points:
      type: array
      items:
        type: string
  required: [summary, key_points]
---

Summarize the following text:

{{ . }}
```

### Front Matter Fields

| Field             | Required | Description |
|-------------------|----------|-------------|
| `name`            | —        | Unique identifier. Derived from the filename if omitted. |
| `title`           | —        | Human-readable title (min 10 chars). Extracted from the first markdown heading if omitted. |
| `description`     | —        | Longer description of the agent's purpose. |
| `model`           | —        | LLM model name (e.g. `claude-haiku-4-5-20251001`). |
| `provider`        | —        | Provider name (e.g. `anthropic`, `google`, `mistral`). |
| `system_prompt`   | —        | System prompt sent to the model. |
| `input`           | —        | JSON Schema defining the expected input variables. |
| `output`          | —        | JSON Schema defining the structured output format. |
| `tools`           | —        | List of tool names the agent is allowed to use. |
| `thinking`        | —        | Enable thinking/reasoning (`true` or `false`). |
| `thinking_budget` | —        | Token budget for thinking (Anthropic only). |

### Template Functions

| Function  | Signature                    | Description |
|-----------|------------------------------|-------------|
| `json`    | `json <value>`               | Marshals a value to its JSON representation. |
| `default` | `default <fallback> <value>` | Returns `value` unless nil or empty, otherwise `fallback`. |
| `join`    | `join <list> <sep>`          | Joins a list into a string with the given separator. |
| `upper`   | `upper <string>`             | Converts to uppercase. |
| `lower`   | `lower <string>`             | Converts to lowercase. |
| `trim`    | `trim <string>`              | Removes leading and trailing whitespace. |

### Creating and Registering Prompts

Parse a prompt from a markdown file and register it as a builtin:

```go
import "github.com/mutablelogic/go-llm/pkg/agent"

// From a file on disk
meta, err := agent.ReadFile("etc/agent/summarize.md")
if err != nil {
    log.Fatal(err)
}
if err = tk.AddPrompt(meta); err != nil {
    log.Fatal(err)
}
```

Parse from an `io.Reader` (e.g. an embedded file):

```go
//go:embed etc/agent/summarize.md
var summarizeMD []byte

meta, err := agent.Read(bytes.NewReader(summarizeMD))
if err != nil {
    log.Fatal(err)
}
if err = tk.AddPrompt(meta); err != nil {
    log.Fatal(err)
}
```

Construct a prompt directly from a `schema.AgentMeta` literal (or unmarshal from JSON). `schema.AgentMeta` implements `llm.Prompt`, so it is passed directly to `AddPrompt`:

```go
import "github.com/mutablelogic/go-llm/pkg/schema"

meta := schema.AgentMeta{
    Name:     "greet",
    Title:    "Greet the user",
    Template: "Say hello to {{ .name }}.",
}
if err := tk.AddPrompt(meta); err != nil {
    log.Fatal(err)
}

// Or unmarshal from JSON:
var meta schema.AgentMeta
if err := json.Unmarshal(jsonBytes, &meta); err != nil {
    log.Fatal(err)
}
if err = tk.AddPrompt(meta); err != nil {
    log.Fatal(err)
}
```

Remove a builtin prompt by name:

```go
if err := tk.RemoveBuiltin("summarize"); err != nil {
    log.Fatal(err)
}
```

### Running Prompts

Prompts are executed via the toolkit, which delegates to the handler (typically the manager). The manager renders the template, selects a model, and runs the agent loop:

```go
// Look up a builtin or connector-supplied prompt by name.
prompt := tk.Lookup(ctx, "summarize") // returns nil if not found

// Pass a plain text string as input.
text := "The quick brown fox..."
result, err := tk.Call(ctx, prompt, toolkit.TextResource(text, ""))

// With optional additional attachments.
result, err = tk.Call(ctx, prompt,
    toolkit.TextResource(text, "Text to summarize"),
    attachment, // optional extra resource
)

// Call also accepts an llm.Tool directly.
result, err = tk.Call(ctx, tk.Lookup(ctx, "my_tool"), toolkit.JSONResource(inputMap, ""))
```

The manager:

1. Renders the prompt's Go template against the variables in the first JSON resource.
2. Selects a model using the prompt's `model`/`provider` front matter, falling back to the manager's default.
3. Runs an LLM agent loop, passing any remaining resources as message attachments.
4. Returns the final output as an `llm.Resource`.

**Errors:**

* `llm.ErrNotFound` — prompt does not exist, or the requested model/provider is not registered.
* `llm.ErrBadParameter` — no handler was configured on the toolkit (the toolkit has no connection to a manager that can run models).

> **TODO:** Define a maximum call depth to prevent infinite recursion when a prompt's tool list includes other prompts that in turn call back into the toolkit.

## Tools

Every tool must satisfy the `llm.Tool` interface:

```go
type Tool interface {
    // unique identifier (letters, digits, underscores only)
    Name()         string          

    // human-readable description of the tool's purpose and behavior
    Description()  string

    // JSON Schema defining the expected input; must be an object.
    InputSchema()  json.RawMessage

    // JSON Schema defining the expected output, or an empty string if no output is defined.
    OutputSchema() json.RawMessage 

    // Optional hints about the tool's behavior.
    Meta()         llm.ToolMeta

    // Run executes the tool with the given JSON input, returning an optional output resource.
    Run(ctx context.Context, input json.RawMessage) (llm.Resource, error)
}

// Return optional hints about the tool's behaviour. All fields are advisory:
type ToolMeta struct {
    // Title is a human-readable display name (takes precedence over Name).
    Title string

    // ReadOnlyHint indicates the tool does not modify its environment.
    ReadOnlyHint bool

    // DestructiveHint, when non-nil and true, indicates the tool may perform
    // destructive updates. Meaningful only when ReadOnlyHint is false.
    DestructiveHint *bool

    // IdempotentHint indicates repeated identical calls have no additional effect.
    // Meaningful only when ReadOnlyHint is false.
    IdempotentHint bool

    // OpenWorldHint, when non-nil and true, indicates the tool may interact
    // with external entities outside a closed domain (e.g. web search).
    OpenWorldHint *bool
}
```

`Run` returns an `llm.Resource`, or `nil` if there is no output. Use `toolkit.JSONResource` for JSON output.

Embed `toolkit.DefaultTool` to get no-op implementations of `OutputSchema` and `Meta`, reducing boilerplate:

```go
type MyTool struct {
    toolkit.DefaultTool
}

func (t *MyTool) Name()        string { return "my_tool" }
func (t *MyTool) Description() string { return "Does something useful." }

func (t *MyTool) InputSchema() json.RawMessage {
    // return your JSON schema here
}

func (t *MyTool) Run(ctx context.Context, input json.RawMessage) (llm.Resource, error) {
    return toolkit.JSONResource(map[string]string{"result": "ok"}, "")
}
```

### Session Context

A `Session` provides per-call services injected into the `ctx` passed to `Run`. Retrieve it with:

```go
sess := toolkit.Session(ctx)
```

It always returns a valid non-nil session — in unit tests where no session is injected a no-op is returned.

```go
type Session interface {
    // ID returns the unique identifier for this client session.
    ID() string

    // ClientInfo returns the name and version of the connected MCP client.
    // Returns nil when called outside an MCP session (e.g. in unit tests).
    ClientInfo() *mcp.Implementation

    // Capabilities returns the capabilities advertised by the client.
    // Returns nil when called outside an MCP session.
    Capabilities() *mcp.ClientCapabilities

    // Meta returns the _meta map sent by the client in this tool call.
    // Returns nil when no _meta was provided.
    Meta() map[string]any

    // Logger returns a slog.Logger whose output is forwarded to the client
    // as MCP notifications/message events.
    Logger() *slog.Logger

    // Progress sends a progress notification back to the caller.
    // progress is the amount completed so far; total is the total expected
    // (0 means unknown); message is an optional human-readable status string.
    Progress(progress, total float64, message string) error
}
```

Example:

```go
func (t *MyTool) Run(ctx context.Context, input json.RawMessage) (llm.Resource, error) {
    sess := toolkit.Session(ctx)
    sess.Logger().Info("tool called", "client", sess.ClientInfo())
    sess.Progress(0.5, 1.0, "halfway")
    // ...
}
```

### Tracing

Pass an OpenTelemetry `trace.Tracer` to `NewToolkit` with `WithTracer`:

```go
import "go.opentelemetry.io/otel/trace"

tk, err := toolkit.New(
    toolkit.WithTool(myTool1, myTool2),
    toolkit.WithHandler(myHandler),
    toolkit.WithTracer(tracer),
)
```

When a tracer is configured, the toolkit starts a span named after the tool before calling its `Run` method and embeds it into the `ctx`. Inside `Run`, retrieve the active span via the standard OpenTelemetry API to create sub-spans or add attributes:

```go
import (
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

func (t *MyTool) Run(ctx context.Context, input json.RawMessage) (llm.Resource, error) {
    // Retrieve the span started by the toolkit.
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(attribute.String("input.size", strconv.Itoa(len(input))))

    // Start a child span for an expensive sub-operation.
    ctx, child := trace.SpanFromContext(ctx).TracerProvider().Tracer("my_tool").Start(ctx, "fetch")
    defer child.End()

    // ...
}
```

If no tracer is configured, `trace.SpanFromContext` returns a no-op span, so tool code is always safe to call without guards.

> **TODO:** Support distributed trace propagation from MCP clients. When a client injects W3C `traceparent`/`tracestate` headers into the `_meta` map of a `tools/call` request, the toolkit should extract the remote span context via `propagator.Extract(ctx, metaCarrier(sess.Meta()))` before starting the tool's span — making the tool's execution a child of the client's trace rather than a new root.

## Resources

Every resource satisfies the `llm.Resource` interface:

```go
type Resource interface {
    // URI returns the unique identifier of the resource. It must be an absolute
    // URI with a non-empty scheme (e.g. "file:///path/to/file", "data:application/json").
    URI() string

    // Name returns a human-readable name for the resource.
    Name() string

    // Description returns an optional description of the resource.
    Description() string

    // Type returns the MIME type of the resource content, or an empty string if unknown.
    Type() string

    // Read returns the raw bytes of the resource content.
    Read(ctx context.Context) ([]byte, error)
}
```

### Built-in Resource Constructors

Three constructors create transient resources:

| Constructor | MIME type | Error |
|---|---|---|
| `TextResource(text, description string) llm.Resource` | `text/plain` | — |
| `BinaryResource(r io.Reader, description string) (llm.Resource, error)` | detected from content | read failure |
| `JSONResource(v any, description string) (llm.Resource, error)` | `application/json` | marshal failure |

`TextResource` wraps a plain-text string:

```go
return toolkit.TextResource("hello, world", "A greeting message"), nil
```

`BinaryResource` reads all bytes from an `io.Reader` eagerly and detects the MIME type from the content:

```go
f, _ := os.Open("image.png")
defer f.Close()
res, err := toolkit.BinaryResource(f, "Screenshot of the dashboard")

// No description needed.
res, err = toolkit.BinaryResource(f, "")
```

`JSONResource` accepts either a `json.RawMessage` / `[]byte` (used as-is) or any Go value (marshalled with `encoding/json`):

```go
// From a Go struct — marshalled automatically.
res, err := toolkit.JSONResource(map[string]string{"result": "ok"}, "Tool output")

// From pre-marshalled bytes — no re-encoding.
res, err = toolkit.JSONResource(json.RawMessage(`{"result":"ok"}`), "")
```

All three constructors set `URI()` to a `data:` URI (e.g. `data:text/plain`, `data:image/png`, `data:application/json`). These are transient identifiers, not named addressable resources.

### Implementing a Custom Resource

To expose a named, addressable resource — for example a file, a database record, or a live sensor
reading — implement `llm.Resource` directly:

```go
type FileResource struct {
    path string
}

func (r *FileResource) URI()         string { return "file://" + r.path }
func (r *FileResource) Name()        string { return filepath.Base(r.path) }
func (r *FileResource) Description() string { return "" }
func (r *FileResource) Type()        string { return "text/plain" }

func (r *FileResource) Read(ctx context.Context) ([]byte, error) {
    return os.ReadFile(r.path)
}
```

### Resources as Tool Outputs

`Run` returns `(llm.Resource, error)`. Return `nil` when the tool produces no output:

```go
func (t *MyTool) Run(ctx context.Context, input json.RawMessage) (llm.Resource, error) {
    return toolkit.JSONResource(map[string]string{"result": "ok"}, "")
}
```

### Resources as Tool Inputs

Additional resources can be passed to `tk.Call` — they are forwarded to the tool's `Run` method
alongside the primary JSON input. For tools, these arrive via the session context; for prompts,
the first resource is used as template variables and any remaining resources are attached to the
generated message.

```go
// Pass a previously produced resource as context for the next call.
result, err := tk.Call(ctx, tk.Lookup(ctx, "summarise"), previousResource)
```

### Builtin Static Resources

Builtin resources (static, pre-known data blobs) can be registered with `AddResource` alongside
tools and prompts:

```go
if err := tk.AddResource(&FileResource{path: "/etc/motd"}); err != nil {
    log.Fatal(err)
}
```

They appear in `tk.List` and are retrievable by URI via `tk.Lookup`.

### Connector Resources

Resources advertised by a remote MCP server are managed automatically. When the server notifies
the toolkit that its resource list has changed, `ToolkitHandler.OnResourceListChanged` is called.
When a specific resource's content is updated, `ToolkitHandler.OnResourceUpdated` is called with
the resource's URI. The handler can call `c.ListResources(ctx)` directly to retrieve the current
list from the connector.

## Toolkit as MCP Server

> **TODO:** This section describes planned functionality that has not yet been implemented.

A toolkit can serve as the capability backend for an MCP server. The toolkit's `List` and `Lookup`/`Call` surface maps directly onto the MCP protocol messages a server must handle:

| MCP request | Toolkit equivalent |
|---|---|
| `tools/list` | `tk.List(ctx, ListRequest{Tools: true})` |
| `tools/call` | `tk.Call(ctx, tk.Lookup(ctx, name), ...)` |
| `prompts/list` | `tk.List(ctx, ListRequest{Prompts: true})` |
| `prompts/get` + run | `tk.Call(ctx, tk.Lookup(ctx, name), ...)` |
| `resources/list` | `tk.List(ctx, ListRequest{Resources: true})` |
| `resources/read` | `tk.Lookup(ctx, uri).(llm.Resource).Read(ctx)` |

An MCP server implementation holds a `*Toolkit` and delegates all capability requests to it. This exposes an arbitrary mix of builtins, upstream MCP connectors, and manager-backed user prompts to any MCP client — the toolkit acts as a protocol-neutral aggregation layer that the server wraps with SSE or stdio transport.

```go
type MyMCPServer struct {
    tk *toolkit.Toolkit
}

// Handle a tools/call request from an MCP client.
func (s *MyMCPServer) CallTool(ctx context.Context, name string, input json.RawMessage) (llm.Resource, error) {
    item := s.tk.Lookup(ctx, name)
    if item == nil {
        return nil, llm.ErrNotFound
    }
    return s.tk.Call(ctx, item, toolkit.JSONResource(input, ""))
}

// Handle a tools/list request from an MCP client.
func (s *MyMCPServer) ListTools(ctx context.Context) ([]llm.Tool, error) {
    resp := s.tk.List(ctx, toolkit.ListRequest{Tools: true})
    return resp.Tools, nil
}
```

The `ToolkitHandler` callbacks also align with the MCP server's responsibility to push change notifications to connected clients:

| Toolkit callback | MCP notification to send |
|---|---|
| `OnToolListChanged` | `notifications/tools/list_changed` |
| `OnPromptListChanged` | `notifications/prompts/list_changed` |
| `OnResourceListChanged` | `notifications/resources/list_changed` |
| `OnResourceUpdated` | `notifications/resources/updated` |

This means when an upstream MCP connector reconnects and its tool list changes, the server can automatically fan the notification out to all of its own connected clients without any additional bookkeeping.

## Using a Toolkit with Generation

Pass the toolkit to a generation call via `toolkit.WithToolkit`:

```go
resp, err := model.Generate(ctx, prompt,
    toolkit.WithToolkit(tk),
)
```

To add individual tools without a toolkit, use `toolkit.WithTool`:

```go
resp, err := model.Generate(ctx, prompt,
    toolkit.WithTool(myTool),
)
```

## Structured Output Tool

`OutputTool` lets you capture structured output from a model that doesn't support a native response schema alongside function calling (e.g. Gemini). The model is instructed to call `submit_output` with its final answer.

```go
s, _ := jsonschema.Reflect(MyOutput{})
outputTool := toolkit.NewOutputTool(s)
if err := tk.AddTool(outputTool); err != nil {
    log.Fatal(err)
}
```

The constant `toolkit.OutputToolInstruction` provides a ready-made system prompt addition that directs the model to call `submit_output` with its final answer.

## Toolkit Interface

The full surface of the `Toolkit` type, for implementation reference:

```go
// Option configures a Toolkit at construction time.
type Option func(*Toolkit) error

// WithTool registers one or more builtin tools with the toolkit at construction time.
func WithTool(items ...llm.Tool) Option

// WithPrompt registers one or more builtin prompts with the toolkit at construction time.
func WithPrompt(items ...llm.Prompt) Option

// WithResource registers one or more builtin resources with the toolkit at construction time.
func WithResource(items ...llm.Resource) Option

// WithHandler sets the ToolkitHandler that receives connector lifecycle callbacks,
// executes prompts, serves the "user" namespace, and creates connectors.
func WithHandler(h ToolkitHandler) Option

// WithTracer sets an OpenTelemetry tracer. The toolkit starts a span named after
// the tool before each Run call and embeds it into the ctx.
func WithTracer(t trace.Tracer) Option

// NewToolkit creates a new Toolkit with the given options.
func New(opts ...Option) (*Toolkit, error)

// Toolkit aggregates tools, prompts, and resources from builtins, remote MCP
// connectors, and the manager-backed "user" namespace.
type Toolkit interface {
    // AddTool registers one or more builtin tools.
    AddTool(...llm.Tool) error

    // AddPrompt registers one or more builtin prompts.
    // Any type implementing llm.Prompt is accepted, including schema.AgentMeta.
    AddPrompt(...llm.Prompt) error

    // AddResource registers one or more builtin resources.
    AddResource(...llm.Resource) error

    // RemoveBuiltin removes a previously registered builtin tool by name,
    // prompt by name, or resource by URI.
    // Returns an error if the identifier matches zero or more than one item.
    RemoveBuiltin(string) error

    // AddConnector registers a remote MCP server. The namespace is inferred from
    // the server (e.g. the hostname or last path segment of the URL). Safe to call
    // before or while Run is active; the connector starts immediately if Run is
    // already running.
    AddConnector(string) error

    // AddConnectorNS registers a remote MCP server under an explicit namespace.
    // Safe to call before or while Run is active; the connector starts immediately
    // if Run is already running.
    AddConnectorNS(namespace, url string) error

    // RemoveConnector removes a connector by URL. Safe to call before or
    // while Run is active; the connector is stopped immediately if running.
    RemoveConnector(string) error

    // Run starts all queued connectors and blocks until ctx is cancelled.
    // It closes the toolkit and waits for all connectors to finish on return.
    Run(context.Context) error

    // Lookup finds a tool, prompt, or resource by name, namespace.name, URI,
    // or URI#namespace. Returns nil if nothing matches.
    Lookup(context.Context, string) any

    // List returns tools, prompts, and resources matching the request.
    List(context.Context, ListRequest) (*ListResponse, error)

    // Call executes a tool or prompt, passing optional resource arguments.
    // For tools, resources are made available via the session context.
    // For prompts, the first resource supplies template variables and any
    // remaining resources are attached to the generated message.
    Call(context.Context, any, ...llm.Resource) (llm.Resource, error)
}
```
