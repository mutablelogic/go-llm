# Changes

`GET /api/changes` streams the server's changes as server-sent events, so clients such as the web UI stay up to date. It's [go-server's broadcaster](https://github.com/mutablelogic/go-server/blob/main/broadcaster/httphandler/README.md), which documents the stream itself.

```bash
llm changes
llm changes --schema=llm --table=session
```

Each change has an `action` (`create`, `update` or `delete`), a `schema`, a `table`, a `row`, and sometimes `data`.

| Schema | Table | Row | Data |
|---|---|---|---|
| `llm` | `provider` | Provider name | Updated when it can, or can't, be reached, or its models change |
| `llm` | `session` | Conversation ID | `{"serial":3}` when it changes, or `{"status":"replying"}` (`typing`, `thinking`, `replying`, `idle`) |
| `llm` | `event` | Outcome ID | The outcome of an event, such as a heartbeat's run |
| `llm` | `tool` | Tool name, such as `agent:segment` | |
| `memory` | `detail` | Field, such as `location` | The detail |
| `memory` | `memory` | Memory key | `{"version":"…"}` |
| `heartbeat` | `heartbeat` | Heartbeat ID | |
| `resource` | `resource` | Resource hash | |
| `mcp` | `connector` | Connector name | Connected, or needs signing in |
| `workspace` | `workspace` | Workspace name | Added or removed |
| `workspace` | `resource` | Workspace name | `{"uri":"resource://…"}` of the folder or resource |
