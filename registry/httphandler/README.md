# LLM Registry

## Provider Operations

Providers are LLM backends, such as Ollama, in the registry. Each has a name in the registry, unique across its providers, and a type, so there can be more than one provider of a type, such as two Ollama servers. A model is referred to by its name, or by its provider's name and its name, separated by a colon, such as `my-ollama:llama3.2:latest`.

The types are:

- `ollama`: an Ollama server. A `url` without a scheme, such as `localhost`, uses `http` and Ollama's port, 11434.
- `gowhisper`: a [go-whisper](https://github.com/mutablelogic/go-whisper) server, whose whisper.cpp models transcribe speech, and which downloads and deletes them, without an `api_key`. A `url` without a scheme, such as `localhost`, uses `http` and go-whisper's port, 8081, and one without a path uses `/api/whisper`. Only go-whisper's own models are listed, not those of the services it forwards to.
- `openai`: OpenAI, which is the default `url`, or a server with an OpenAI-compatible API, such as DeepSeek, Mistral (`https://api.mistral.ai/v1`), vLLM or LM Studio.
- `elevenlabs`: ElevenLabs, which is the default `url`, whose Scribe models transcribe speech, with an API key, which it needs, and is sent in its `xi-api-key` header, rather than as a bearer token.
- `eliza`: a mock provider for testing, which has no `url` or API key.

The registry isn't stored, so providers which are created are lost when the server stops, unless the server syncs them with a store, such as a file of providers, when they're kept, and providers which are created, changed or deleted in the store are in the registry. An API key is sent as a bearer token, and is never returned. An API key can be a credential's placeholder, such as `${OPENAI_KEY}`, which is replaced with the credential, which is the environment variable with that name, unless the server has another credentials provider. A provider is pinged when it's listed or got, unless it was pinged within the last minute, and has an `error` when it can't be reached. A provider's models are those whose names match one of its `include` patterns, or all of them when it has none, and none of its `exclude` patterns, which are regular expressions, such as `^gpt-`. They're cached, and listed again after six hours, or every minute for a provider which downloads models, such as Ollama, or when they couldn't be listed.

### GET /llm/provider

List the providers, in order of their names, with what each can do, such as generate messages, or download models, and an `error` for each which can't be reached. The `provider` parameter returns only the providers of that type, such as `ollama`. Returns 400 if a parameter is invalid.

### POST /llm/provider

Create a provider of a type, with its `name` in the registry, which is an identifier, of up to 64 letters, numbers, underscores or hyphens, starting with a letter, or its type when it's omitted, and its `url` and `api_key`, which aren't needed for every type, and the `include` and `exclude` patterns of the names of its models. The provider is checked to be reachable before it's added. Returns the created provider. Returns 400 if the request body is invalid, the type is unknown, a pattern isn't a valid regular expression, or the API key's placeholder names a credential which isn't set, 409 if a provider with that name exists already, or 503 if the provider can't be reached.

### GET /llm/provider/{name}

Get a provider by name. Returns 404 if there is no provider with that name.

### PATCH /llm/provider/{name}

Update a provider by name, replacing its `url`, `include` and `exclude` patterns with those in the request, so a field which is omitted is cleared, or is the type's default. Its `api_key` is replaced when the request has one, and is otherwise kept, as it isn't returned, so it can't be sent back. A `url` which is the provider's, as it's returned, with its password redacted, keeps the URL it has, with its password, so it can be sent back, such as to change only the patterns. Its name and type can't be changed. The provider is created again, whether it can be reached or not, with an `error` when it can't, and a provider whose fields are the same is left as it is. Returns the updated provider. Returns 400 if the request body is invalid, a pattern isn't a valid regular expression, or the API key's placeholder names a credential which isn't set, 404 if there is no provider with that name, or 409 if the provider was added by the server, rather than with the API.

### DELETE /llm/provider/{name}

Delete a provider by name, from the registry. Its models can't be used afterwards. Returns the deleted provider. Returns 404 if there is no provider with that name.

## Model Operations

Models are the models of the providers, which match their `include` and `exclude` patterns, and are cached. A model is referred to by its name, or an alias, or by its provider's name and its name separated by a colon, such as `my-ollama:llama3.2:latest`. A model's name can itself have a colon, such as Ollama's `llama3.2:latest`, so `llama3.2:latest` and `my-ollama:llama3.2:latest` both refer to it, when only one provider has it. In a path, a model's name is one segment, so a `/` in it is escaped, as `%2F`, such as `my-ollama:x%2Fflux2-klein:latest` for `my-ollama:x/flux2-klein:latest`, or `meta-llama%2FLlama-3.1-8B`.

### GET /llm/model

List the models, in order of their providers' names, and then their names. Each model has the name of the `provider` it's from, and its `canonical` name, which is its provider's name and its name. The `provider` parameter returns only the models of the provider with that name. The models of a provider which can't be listed, such as when it can't be reached, and none are cached, are left out. Returns 400 if a parameter is invalid, 404 if there's no provider with the name, or 503 if no provider's models can be listed.

### POST /llm/model

Load a model with a provider which loads models, such as Ollama, which downloads it when it isn't stored. The request body has the `name` of the model, and the name of the `provider` to load it with, which is needed when more than one provider loads models. When the request accepts `text/event-stream`, the download's progress is streamed as server-sent events: `progress` events, with its `status` and the `percent` complete, followed by a `result` event with the model, or an `error` event. Otherwise the response is the model, once it's loaded. Returns 400 if the request body is invalid, the name is missing, or more than one provider loads models, and the request has no provider, 404 if there's no provider with the name, 406 if the response can't be any of the accepted types, 501 if no provider loads models, or not the provider in the request, or 503 if the model can't be loaded.

### GET /llm/model/{name}

Get a model by name, or alias, or by provider and name, such as `my-ollama:llama3.2:latest`. Returns 404 if no provider has the model, 409 if more than one provider has the model, when the error lists the provider and name to use for each, or 503 if no provider has the model, but a provider which could can't be reached.

### DELETE /llm/model/{name}

Delete a model by name, or alias, or by provider and name, such as `my-ollama:gemma:latest`, from its provider's storage, with a provider which deletes models, such as Ollama. Returns the deleted model. Returns 404 if no provider has the model, 409 if more than one provider has the model, 501 if the model's provider can't delete models, or 503 if the model couldn't be deleted, or a provider which could have it can't be reached.

### POST /llm/model/{name}/unload

Unload a model by name, or alias, or by provider and name, such as `my-ollama:gemma:latest`, from its provider's memory, keeping it stored, so it's loaded again when it's next used. Returns the unloaded model. Returns 404 if no provider has the model, 409 if more than one provider has the model, 501 if the model's provider doesn't load models, or can't unload them, or 503 if the model couldn't be unloaded, or a provider which could have it can't be reached.

## Session Operations

Sessions are conversations with models, which the server keeps in its session store, such as files, or a database which other servers share, so they're kept when it restarts. A server without a session store doesn't have them, so they're not in its API. Sessions are returned without their messages, which are listed on their own, as there can be many. A session has its `id`, which the store sets, and its `serial`, which the store changes each time the session is stored, so a change is made to the session as it was last seen. It has its `title`, for people, which it needn't have, such as an event's, whose session takes the event's title, such as a heartbeat's, when it's created, and when it's changed. It has the settings of how its model is asked: its `model`, `system` prompt, the `tools` it can call, its `opt` options and the `schema` of its reply, as for `POST /llm/ask`, which it needn't all have, such as when it inherits its model from another session. It has its `messages`, each with its `tokens`, and its `usage`: the tokens used by every request to its model, and, as of the last, the tokens of its conversation, of the messages sent to the model (`input`), of its replies (`output`), and of what's sent with them (`overhead`), such as the system prompt and the tools' definitions. It has `event`, which is true when it's an event's, such as a heartbeat's, which runs in it, which only the registry sets, so the API ignores it, and which isn't chatted in, so chatting in it returns 403. It has its `status`, which isn't stored: `typing` while someone types a message in it, `replying` while its model replies, or `idle`. Each change to a session, and its status, is an event, which `GET /events` streams, so a client, such as a second chat window on the same session, sees it.

### GET /llm/session

List the sessions, without their messages, most recently modified first. The `offset` parameter skips that many sessions, and the `limit` parameter returns at most that many. The `count` is how many sessions there are. Returns 400 if a parameter is invalid.

### POST /llm/session

Create a session with the settings in the request body, and no messages. Its `title` is kept without the space around it, or isn't, when it's empty. Its `model`, when it has one, must be a model in the registry, and is kept by its canonical name, such as `eliza:eliza-1966-en` for `eliza`, and its `tools` must be the server's, and are kept by their canonical names, such as `weatherapi:current` for `current`, apart from a namespace and `*`, such as `memory:*`, or `*`, which are kept as they are. Returns the created session. Returns 400 if the request body is invalid, the `schema` isn't a JSON schema, or an option is the `system_prompt` or `json_schema`, which are fields, 404 if the model, or a tool, isn't found, or 409 if the name of the model, or a tool, is ambiguous.

### GET /llm/session/{id}

Get a session by its identifier, without its messages, which are listed with `GET /llm/session/{id}/messages`. Returns 404 if there is no session with that identifier.

### GET /llm/session/{id}/messages

List a session's messages, oldest first, each with its `tokens`. The `role` parameter returns only the messages of that role, such as `user`, `assistant` or `tool`, the `offset` parameter skips that many messages, and the `limit` parameter returns at most that many. The `count` is how many messages there are, of the role. Returns 400 if a parameter is invalid, or 404 if there is no session with that identifier.

### DELETE /llm/session/{id}/messages

Forget a session's messages, keeping its settings, such as its model, system prompt and tools, so its conversation starts again. The tokens of its conversation, which are its `input`, `output` and `overhead`, are none, and the tokens used by every request to its model are kept. The session is updated, so clients which watch it load it again. A reply which is being waited for, as they're forgotten, isn't stored. Returns the session, without its messages. Returns 400 if the identifier isn't a UUID, 404 if there is no session with that identifier, or 409 if it was changed as its messages were forgotten.

### PATCH /llm/session/{id}

Change a session's settings, keeping its messages. A field which is omitted is kept: `title`, `model` and `system` are replaced when they're set, and cleared when they're empty; `tools` are replaced when they're set, and cleared with `[]`; the `schema` is replaced when it's set, and cleared with `null`; and each option in `opt` is set, or removed when it's `null`, so the others are kept. A `model` or `tools` which are set are checked, and kept by their canonical names, as they are when a session is created. Returns the updated session. Returns 400 if the request body is invalid, it changes nothing, or the settings, as changed, aren't valid, 404 if there is no session with that identifier, or the model, or a tool, isn't found, or 409 if it was changed as it was updated, or the name of the model, or a tool, is ambiguous.

### PUT /llm/session/{id}/status

Set a session's `status`, in the request body, to `typing`, as someone types a message, or `idle`, when they stop. It's `typing` until 10 seconds after it was last set, so a client which is still typing sets it again before then, and one which goes away doesn't leave it typing. While its model replies, it's `replying`, whatever it's set to, and sending a message ends `typing`. Returns the session, with its status. Returns 400 if the request body is invalid, or the status isn't `typing` or `idle`, or 404 if there is no session with that identifier.

### DELETE /llm/session/{id}

Delete a session by its identifier, such as an event's, which is created again, without its messages, when the event runs again. Returns the deleted session. Returns 404 if there is no session with that identifier, or 409 if its model is replying, such as as an event runs in it, or it was changed as it was deleted.

### GET /llm/event

List the outcomes of events, such as heartbeats which fired, which were run, the most recent first, with how many there are. Each has the session it `run` in, which has what it did, the `session` it's for, its `title`, its `message`, its `importance`, which is `ignore`, `notify` or `alert`, its `status`, and when it was `scheduled`, and `fired`. Its `status` is `scheduled`, until it's run, `failed`, when it couldn't be, with the error as its message, `ignored`, when there's nothing worth telling the person, and otherwise `pending`, until it's `notified`, as the person was told, or `delivered`, into the session it's for. An event which was triggered has its `source`. With `session`, only the events for the session are listed, and with `status`, only those of the status. From the `offset`, and at most the `limit`. When an event's outcome is created, changed, or deleted, an event is published, whose `table` is `event`, whose `row` is its identifier, and whose data is the `session` it's for, and, unless it was deleted, its `serial` and `status`. Returns 400 if the session isn't a UUID, or the status isn't one.

### POST /llm/event

Trigger an event, from a source other than a heartbeat, such as a file which changed, or a webhook, which is scheduled to run, in the background, as a heartbeat which fires is run. The request has the `session` it's for, whose model, and tools, it's run with, its `prompt`, which is the instruction for it, its `title`, which is Event, unless it's given, its `data`, such as a file's path, which is any JSON, and is given to the agent as information, never as instructions, when it happened, as `scheduled`, which is now, unless it's given, and its `source`, such as `inbox`, so the events of the source, for the session, run in one session, as a heartbeat's do, rather than each in a session of its own. Returns 202, with `scheduled`, which is true, and the event's outcome, whose `status` is `scheduled`, until it's run, when it's updated, as `pending`, or `ignored`, with its message, or `failed`, with the error, when it couldn't be run, which is published as an event, and which `GET /llm/event/{id}` gets. When an event of the same source is scheduled already, for the session, by this server, the event isn't scheduled, so a burst of them runs once, and at most `--llm.max-events` events, which is 2, unless it's set, run at once, as the others wait, scheduled: returns 200, with `scheduled`, which is false, and the outcome of the event which is. Returns 400 if it has no session, or prompt, or 404 if there's no session.

### GET /llm/event/{id}

Get the outcome of the event with the identifier. Returns 404 if there's none.

### DELETE /llm/event/{id}

Delete the outcome of the event with the identifier, which leaves the session it ran in as it is. Returns the event, as it was. Returns 404 if there's none, or 409 if it was changed as it was deleted.

## Agent Operations

Agents are prompts, which is what MCP calls them, and how a model is asked them, which the server keeps in its agent store, such as files, or a database which other servers share. A server without an agent store doesn't have them, so they're not in its API. An agent has its `name`, which is an identifier, without two underscores, or starting or ending with one, which refers to it, its `title` and `description`, the JSON schema of its `input`, which is an object, and its `prompt`, a Go template of the message to the model, which is executed with the input. It has the settings of how its model is asked, as a session has: its `model`, `system` prompt, the `tools` it can call, its `opt` options and the `schema` of its reply, which it needn't all have. An agent which is `internal`, such as the `event` agent, which runs a heartbeat which fires, is only run by the server, as its input is trusted, so it isn't a tool which models can call. An agent which is `builtin`, such as `translate`, is one the server has, which it creates as it is each time it starts, so it can't be replaced, changed or deleted, which returns 403, but its definition can be copied under another name, which isn't built in, as the API ignores `builtin`. It has its `serial`, which the store changes each time it's stored, and is never one the agent had, so a change is made to it as it was last seen.

### GET /llm/agent

List the agents, in the order the server's agent store keeps them. The `offset` parameter skips that many agents, and the `limit` parameter returns at most that many. The `count` is how many agents there are. Returns 400 if a parameter is invalid.

### POST /llm/agent

Create an agent with the definition in the request body, or, when there's an agent with its name, replace it with the definition, as it is, rather than merging them, when it's changed, or keep it as it is, when it isn't, so the same definitions can be created each time, such as when a server starts, which is JSON, or YAML, such as a file of an agent's definition, with `Content-Type: application/yaml`, by the same names, so its `input`, `opt` and `schema` are as they'd be in JSON. In YAML, a name which isn't one of its fields, such as `system_prompt` rather than `system`, is an error. Its `model`, when it has one, must be a model in the registry, and is kept by its canonical name, and its `tools` must be the server's, and are kept by their canonical names, as a session's are. Returns the agent, with 201 when it's created, and 200 when it's replaced, or kept. Returns 400 if the request body is invalid, the name isn't valid, the `input` isn't a JSON schema of an object, the `prompt` isn't a template, the `schema` isn't a JSON schema, or an option is a field, 404 if the model, or a tool, isn't found, 409 if the agent was changed as it was replaced, or the name of the model, or a tool, is ambiguous, or 413 if the request body is larger than 1 MiB.

### GET /llm/agent/{name}

Get an agent by its name, as JSON, with its `serial`, or, when YAML is accepted, such as with `Accept: application/yaml`, its definition, as YAML, as a file of it is: by the same names as JSON, in the same order, with text of more than one line, such as its `prompt`, as a literal block, and without its `serial`, or when it was created or modified, so it can be created again, as it is. Returns 404 if there is no agent with that name, or 406 if neither JSON nor YAML is accepted.

### PATCH /llm/agent/{name}

Change an agent's definition, with the changes in the request body, which is JSON, or YAML, as when it's created. A field which is omitted is kept: `title`, `description`, `prompt`, `model` and `system` are replaced when they're set, and cleared when they're empty; `tools` are replaced when they're set, and cleared with `[]`; the `input` and `schema` are replaced when they're set, and cleared with `null`; and each option in `opt` is set, or removed when it's `null`. Its `name` isn't changed: the body has no `name`, or the agent's, such as when it's the agent's whole definition, from `GET /llm/agent/{name}` as YAML. The definition, as changed, must be valid, as when it's created. Returns the updated agent, as JSON, or its definition, as YAML, when YAML is accepted. Returns 400 if the request body is invalid, has another `name`, changes nothing, or the definition, as changed, isn't valid, 404 if there is no agent with that name, or the model, or a tool, isn't found, 406 if neither JSON nor YAML is accepted, 409 if it was changed as it was updated, or the name of the model, or a tool, is ambiguous, or 413 if the request body is larger than 1 MiB.

### DELETE /llm/agent/{name}

Delete an agent by its name. Returns the deleted agent. Returns 404 if there is no agent with that name, or 409 if it was changed as it was deleted.

## Inference Operations

Inference is asking a model to do something with messages, such as embed them. A model is referred to by its name, or an alias, or by its provider's name and its name separated by a colon, and its provider must be able to do it, or the model isn't known not to.

### POST /llm/ask

Ask a model for its reply to the `messages`, which include earlier turns of the conversation, and the results of the tools the model called. A message's `attachment`, such as an image, or a PDF, has its `type` and its `data`, encoded as base64, or its `url`, which is a resource's URI, `resource://<hash>`, which is a resource on its own, whose content is read as it's sent to the model, and whose type is the resource's, unless it's given. Returns 404 if there's no resource on its own with the URI, or 501 if the server has no resources.

- `model`: the model, by name or as `provider:name`, such as `ollama:llama3.2:latest`.
- `system`: a system prompt, sent before the messages.
- `tools`: the tools the model can call, from the server's toolkit, such as `weatherapi:current`, or `weatherapi:*` for all the tools in a namespace, which the model calls by their namespace and name separated by two underscores, such as `weatherapi__current`. The reply has the tool calls, rather than their results: the caller runs them, such as with `POST /llm/tool/{name}`, and asks again, with the reply, and a message from the `tool` with their results.
- `opt`: options, by key, such as `{"temperature":0.7}`, which the model's provider decodes. The system prompt and the schema are fields, rather than options. These options are the agent loop's, which asks the model, and aren't sent to its provider:
  - `max_iterations`: the most times the model is asked for its reply, one or more. Asking a model stops at the tools it calls, so it's asked again only for a reply with a `schema`, three times, unless this says otherwise. Chatting in a session runs the tools, and asks the model again, ten times, unless this says otherwise, when the reply stops with `max_iterations`.
  - `max_depth`: the most agents which run in one another, one or more, as a model calls an agent, as a tool, whose model calls another agent, or itself. It's three, unless the session, or agent, which runs them says otherwise, and an agent can lower it for those it runs, but not raise it. An agent which would run deeper isn't run, and the model is told why, as the tool's error.
  - `tool_timeout`: the most time a tool which the agent loop runs has to return, such as `"2m"`, which is thirty seconds, unless this says otherwise.
- `schema`: the JSON schema of the reply, which the model replies with, by calling a `respond` tool, rather than with text. A model which replies with text is asked once to call it, and a result which doesn't conform to the schema is returned to the model to correct, until it's been asked three times. The reply's `result` is the JSON, when the model replied with it.

Returns the model's reply as the `message`, the `model` which replied, by its canonical name, and the tokens used, by every request to the model. With `text/event-stream` accepted, the reply is streamed as server-sent events: `text` and `thinking` events with the pieces of the reply as they're generated, and then a `result` event with the reply, or an `error` event. Returns 400 if the request is invalid, has no model or messages, an option which is a field, or an invalid schema, or the model's provider rejects it, 404 if the model, or a tool, isn't found, 409 if more than one provider has the model, or a tool's name without a namespace is ambiguous, 413 if the request body is larger than 32 MiB, 501 if the model's provider can't generate messages, or the model is known not to, or not to call tools, think, or see images, when it's asked to, or 503 if the model couldn't reply, such as when its provider fails, or can't be reached.

### POST /llm/session/{id}

Chat: add the `messages` in the request body, such as a question, to a session, and its model's reply to its conversation. An `attachment` whose `url` is a resource's URI, `resource://<hash>`, is the session's resource, or a resource on its own, which is linked to the session, so it's kept with it, and the session keeps its URI, rather than its content, which is read each time it's sent to the model. One whose resource is deleted, with the session, is sent as text which says it isn't available. Returns 404 if there's no resource with the URI, in the session, or on its own. The model is asked as the session's settings say, with its messages, and its options, including the agent loop's, as for `POST /llm/ask`, and the tools it calls are run, unlike with `POST /llm/ask`, whose caller runs them, until it replies, and the conversation is shortened to fit the model's context, without changing the session's messages. The messages, the model's reply, and the results of the tools it called, each with its `tokens`, are added to the session, with its `usage`. Returns what was added, with the session's `serial`, as it was stored, or, when `text/event-stream` is accepted, a stream of `text`, `thinking`, `log` and `progress` events, as the reply is generated, and the tools it calls run, and then a `result` event, with what was added, or an `error` event. A `log` event is a line a tool logs, such as "Getting the current weather for Berlin", and a `progress` event is how a tool is getting on, which replaces the one before, as MCP's progress notification does: its `progress`, which increases, its `total`, when it's known, and its `message`, which is its `status` too, with its `percent`, when the total is known, as a model's download has. An agent, as a tool, has its tools' `log` and `progress` events, and that it's thinking, or replying, as `progress` events. The result of each tool has its `usage`: how long it took, as its `duration`, in nanoseconds, and the tokens it used, such as an agent, which asks a model, which the session's `usage` includes, as it does the agents' they called, however deep. The `duration` of the usage of a reply is how long it took, from when it was asked to when it replied, rather than the sum of its tools', which run at once. Returns 400 if the request body is invalid, it has no messages, or the session has no model, 404 if there is no session with that identifier, or its model, or a tool, isn't found, or 409 if the session was changed as the model replied, when the reply isn't stored.

### POST /llm/embed

Ask a model for a vector for each of the messages, in the same order, with JSON, or for one message, with a `multipart/form-data` form, such as from a web browser. The JSON request has the `model`, the `messages`, each embedded by its `content`, which is its `text`, and its images and audio as `attachment`s, inline, with their `type` and `data`, or by their `url`, which is a resource's URI, `resource://<hash>`, a resource on its own, and the `opt` as an object, such as `{"dimensions":256}`. The form has the same fields as recognizing text does:

- `model`: the model, by name or as `provider:name`, such as `ollama:embeddinggemma-2:latest`, whose provider must embed messages.
- `file`: the message, uploaded as a file, which is its text when the file is text, such as `text/plain` or `text/markdown`, or otherwise an image or audio, for a model which embeds them, such as `embeddinggemma-2` on Ollama. An OpenAI-compatible provider embeds only text. The file's type is used, or detected from its data, or its name, when it's `application/octet-stream`.
- `opt`: the options, as a JSON object.

Returns a vector for each message, the `model` which embedded them, by its canonical name, and the tokens used. Vectors from different models can't be compared, so a model should be kept for the messages which are compared. Returns 400 if the request is invalid, has no messages or model, a message has no text or attachment, the form has no file, or more than one, or the model's provider rejects it, such as for an option the model doesn't support, 404 if no provider has the model, 409 if more than one does, so its canonical name is needed, 413 if the request body is larger than 32 MiB, 501 if the model's provider can't embed messages, or the model is known not to, or 503 if the model couldn't embed the messages, such as when its provider fails, or can't be reached, or its own credentials are rejected.

### POST /llm/recognize

Ask a model for the text in an image, or a document such as a PDF, which is optical character recognition, with JSON, or with a `multipart/form-data` form, such as from a web browser. The JSON request has the `model`, the image or document's `type` and its `data`, encoded as base64, or its `url`, which is a resource's URI, `resource://<hash>`, a resource on its own, whose type is the resource's, unless it's given, and the `opt` as an object. The form has:

- `model`: the model, by name or as `provider:name`, whose provider must recognize text, such as `mistral:mistral-ocr-latest`, or `ollama:deepseek-ocr:latest`.
- `file`: the image or document, uploaded as a file. The file's type is used, or detected from its data, or its name, when it's `application/octet-stream`.
- `opt`: the options, as a JSON object.

The options are the same for every provider, which rejects those it can't do:

- `pages`: the pages of a document to recognize, from zero, as a number, numbers, such as `[0,2]`, or a string, such as `"0,2-4"`. An image is one page, `0`.
- `table_format`: the format of tables in the text, `markdown`, or `html`, which only Mistral can do.
- `regions`: `true` for the regions of each page, such as its paragraphs, titles and tables, which Mistral, and Ollama's `deepseek-ocr`, can do.

Mistral recognizes images and documents, such as PDFs. Ollama's models, such as `deepseek-ocr`, recognize images, each asked with its own prompt, and should be sent images large enough to read their text, such as a page at 150 dpi. Returns the `pages`, in order, each with its `text`, in Markdown, and its `regions`, when they're asked for, each with its `type`, as the model labels it, such as `text` or `table`, its `text`, and its `box`, which is its left, top, right and bottom, each a fraction of the page's width or height, from 0 to 1. It also returns the `model` which recognized it, by its canonical name, and the `usage`, which is the `pages` recognized, and the tokens used, for providers which report them. Returns 400 if the request is invalid, has no model, type or data, or file, or the model's provider rejects it, such as for a PDF sent to Ollama, or an option it can't do, such as regions for a model which can't recognize them, 404 if no provider has the model, 409 if more than one does, 413 if the request body is larger than 64 MiB, 501 if the model's provider can't recognize text, or the model is known not to, or 503 if the model couldn't recognize the text, such as when its provider fails, or can't be reached.

### POST /llm/speak

Ask a model to speak the `text`, which is text to speech, with the `model`, and the `opt` as an object: its `voice`, by its name, such as `Freya`, the `audio_format`, such as `wav`, `mp3` or `pcm`, the `speed`, where 1 is normal, and `instructions`, which say how it's spoken, such as `Speak calmly`, for a model which takes them. OpenAI's models speak in the `alloy` voice, unless another is given. An Ollama model, such as `phonon`, speaks with Ollama's OpenAI-compatible API, in `wav`, or `pcm`, at the normal speed, and rejects a voice it hasn't, saying which it has. Returns the audio, with its type, such as `audio/wav`, and the model which spoke it, by its canonical name, and the usage, as JSON in its `X-Meta` header. Returns 400 if the request is invalid, has no model or text, or the model's provider rejects an option, such as a voice the model hasn't, 404 if no provider has the model, 409 if more than one does, 413 if the request body is larger than 1 MiB, 501 if the model's provider can't speak text, or the model is known not to, or 503 if the model couldn't speak the text, such as when its provider fails, or can't be reached.

### POST /llm/transcribe

Ask a model for the speech in audio, as text, in the language it was spoken in, with JSON, or with a `multipart/form-data` form, such as from a web browser. The JSON request has the `model`, the audio's `type`, such as `audio/mpeg`, and its `data`, encoded as base64, or its `url`, which is a resource's URI, `resource://<hash>`, a resource on its own, whose type is the resource's, unless it's given, and the `opt` as an object. The form has the same fields as recognizing text does: the `model`, the audio as a `file`, and the `opt`, as a JSON object.

The options are the same for every provider, which rejects those it can't do:

- `language`: the language the speech is in, as an ISO 639-1 code, such as `en`, rather than it being detected.
- `segments`: `true` for the segments of the speech, each with its `start` and `end`, in nanoseconds, and its `text`, which whisper, Mistral, and OpenAI's diarizing models can do.
- `speakers`: `true` for the segments with who spoke them, as the model labels them, such as `A` or `speaker_1`, which Mistral, and OpenAI's diarizing models, such as `gpt-4o-transcribe-diarize`, can do.
- `temperature`: the randomness of the transcript.

When the request accepts `text/event-stream`, the transcript is streamed as server-sent events as it's transcribed: a `text` event for each piece of its text, as `{"text":"..."}`, a `segment` event for each segment, once it's transcribed, when segments or speakers are asked for, and then a `result` event with the response, or an `error` event. Mistral's models, and OpenAI's other than `whisper-1`, stream the transcript, and a model which doesn't, such as `whisper-1`, has its text, and segments, as events once it's transcribed. A model which only streams segments, such as `gpt-4o-transcribe-diarize`, has the text of each segment as a `text` event too.

OpenAI's models, such as `whisper-1` and `gpt-4o-transcribe`, and Mistral's, such as `voxtral-mini-latest`, transcribe speech. Returns the `text`, its `language`, as an ISO 639-1 code, when the model detects it, or it's set, the `segments`, when they're asked for, the `model` which transcribed it, by its canonical name, and the `usage`, which is the `seconds` of audio, and the tokens used, as the provider reports them. Returns 400 if the request is invalid, has no model, type or data, or file, or the model's provider rejects it, such as for audio it can't read, or segments or speakers for a model which can't return them, 404 if no provider has the model, 409 if more than one does, 406 if the response can't be any of the accepted types, 413 if the request body is larger than 48 MiB, 501 if the model's provider can't transcribe speech, or the model is known not to, or 503 if the model couldn't transcribe the speech, such as when its provider fails, or can't be reached.

### POST /llm/translate

Ask a model for the speech in audio, in any language, as English text, as transcribing does, whose `language` is `en`, and which has no `language` option. It's streamed as transcribing is, once it's translated, as `whisper-1` doesn't stream. Only OpenAI's `whisper-1` translates speech. Returns the same as transcribing does, and 501 if the model's provider can't translate speech, such as Mistral, or the model is known not to.
