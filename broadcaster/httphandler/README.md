# Broadcaster API

## Event Operations

Events are changes, which are streamed to a client as they happen. Each event has an `action`, which is `create`, `update` or `delete`, the `schema` of the module which changed, the `table` which changed, the `row` which changed, and, when the table says more of the change, its `data`. The registry's and the toolkit's events are of the `llm` schema, the memory's of the `memory` schema, whose `detail` table is its details, and whose `memory` table is its memories, and the heartbeats' of the `heartbeat` schema, whose `heartbeat` table is the heartbeats, and whose `fire` table is those which fire.

The registry's providers are the `provider` table, whose rows are the providers' names: a provider is created or deleted, or updated when whether it can be reached changes, when it's pinged, or its models change, when they're listed again, such as after a model is loaded or deleted.

The registry's sessions are the `session` table, whose rows are the sessions' identifiers: a session is created or deleted, or updated when its settings are, or messages are added to it, when its `data` is its new `serial`, such as `{"serial":3}`, so a client which has that serial has seen the change. A session is also updated when its status changes, when its `data` is its `status`, such as `{"status":"replying"}`: `typing` when a client says someone is typing a message, `replying` while a model replies to one, and `idle` otherwise.

### GET /llm/event

Watch the events, as a stream of server-sent events, until the client closes the connection. Each event is an `event`, whose data is the event, as JSON, such as `{"action":"create","schema":"llm","table":"provider","row":"my-ollama"}`. The stream is watching once it has started, so a client which lists the providers then misses no changes. When the server is stopping, a `shutdown` event is sent, and the stream ends, as there will be no more events. A client which doesn't read its events has up to 16 kept for it, and misses those after them. Returns 406 if the client doesn't accept `text/event-stream`, or 503 if the server is stopping.
