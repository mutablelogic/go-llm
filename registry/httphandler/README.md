# LLM Registry

## Provider Operations

Providers are LLM backends, such as Ollama, in the registry. Each has a name in the registry, unique across its providers, and a type, so there can be more than one provider of a type, such as two Ollama servers. A model is referred to by its name, or by its provider's name and its name, separated by a colon, such as `my-ollama:llama3.2:latest`.

The types are:

- `ollama`: an Ollama server. A `url` without a scheme, such as `localhost`, uses `http` and Ollama's port, 11434.
- `openai`: OpenAI, which is the default `url`, or a server with an OpenAI-compatible API, such as DeepSeek, Mistral (`https://api.mistral.ai/v1`), vLLM or LM Studio.
- `eliza`: a mock provider for testing, which has no `url` or API key.

The registry isn't stored, so providers which are created are lost when the server stops, unless the server syncs them with a store, such as a file of providers, when they're kept, and providers which are created, changed or deleted in the store are in the registry. An API key is sent as a bearer token, and is never returned. An API key can be a credential's placeholder, such as `${OPENAI_KEY}`, which is replaced with the credential, which is the environment variable with that name, unless the server has another credentials provider. A provider is pinged when it's listed or got, unless it was pinged within the last minute, and has an `error` when it can't be reached. A provider's models are those whose names match one of its `include` patterns, or all of them when it has none, and none of its `exclude` patterns, which are regular expressions, such as `^gpt-`. They're cached, and listed again after six hours, or every minute for a provider which downloads models, such as Ollama, or when they couldn't be listed.

### GET /llm/provider

List the providers, in order of their names, with what each can do, such as generate messages, or download models, and an `error` for each which can't be reached. The `provider` parameter returns only the providers of that type, such as `ollama`. Returns 400 if a parameter is invalid.

### POST /llm/provider

Create a provider of a type, with its `name` in the registry, which is an identifier, of up to 64 letters, numbers, underscores or hyphens, starting with a letter, or its type when it's omitted, and its `url` and `api_key`, which aren't needed for every type, and the `include` and `exclude` patterns of the names of its models. The provider is checked to be reachable before it's added. Returns the created provider. Returns 400 if the request body is invalid, the type is unknown, a pattern isn't a valid regular expression, or the API key's placeholder names a credential which isn't set, 409 if a provider with that name exists already, or 503 if the provider can't be reached.

### GET /llm/provider/{name}

Get a provider by name. Returns 404 if there is no provider with that name.

### PATCH /llm/provider/{name}

Update a provider by name, replacing its `url`, `include` and `exclude` patterns with those in the request, so a field which is omitted is cleared, or is the type's default. Its `api_key` is replaced when the request has one, and is otherwise kept, as it isn't returned, so it can't be sent back. Its name and type can't be changed. The provider is created again, whether it can be reached or not, with an `error` when it can't, and a provider whose fields are the same is left as it is. Returns the updated provider. Returns 400 if the request body is invalid, a pattern isn't a valid regular expression, or the API key's placeholder names a credential which isn't set, 404 if there is no provider with that name, or 409 if the provider was added by the server, rather than with the API.

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

## Inference Operations

Inference is asking a model to do something with messages, such as embed them. A model is referred to by its name, or an alias, or by its provider's name and its name separated by a colon, and its provider must be able to do it, or the model isn't known not to.

### POST /llm/ask

Ask a model for its reply to the `messages`, which include earlier turns of the conversation, and the results of the tools the model called.

- `model`: the model, by name or as `provider:name`, such as `ollama:llama3.2:latest`.
- `system`: a system prompt, sent before the messages.
- `tools`: the tools the model can call, from the server's toolkit, such as `weatherapi:current`, or `weatherapi:*` for all the tools in a namespace, which the model calls by their namespace and name separated by an underscore, such as `weatherapi_current`. The reply has the tool calls, rather than their results: the caller runs them, such as with `POST /llm/tool/{name}`, and asks again, with the reply, and a message from the `tool` with their results.
- `opt`: options, by key, such as `{"temperature":0.7}`, which the model's provider decodes. The system prompt and the schema are fields, rather than options.
- `schema`: the JSON schema of the reply, which the model replies with, by calling a `respond` tool, rather than with text. A model which replies with text is asked once to call it, and a result which doesn't conform to the schema is returned to the model to correct, until it's been asked three times. The reply's `result` is the JSON, when the model replied with it.

Returns the model's reply as the `message`, the `model` which replied, by its canonical name, and the tokens used, by every request to the model. With `text/event-stream` accepted, the reply is streamed as server-sent events: `text` and `thinking` events with the pieces of the reply as they're generated, and then a `result` event with the reply, or an `error` event. Returns 400 if the request is invalid, has no model or messages, an option which is a field, or an invalid schema, or the model's provider rejects it, 404 if the model, or a tool, isn't found, 409 if more than one provider has the model, or a tool's name without a namespace is ambiguous, 413 if the request body is larger than 32 MiB, 501 if the model's provider can't generate messages, or the model is known not to, or not to call tools, think, or see images, when it's asked to, or 503 if the model couldn't reply, such as when its provider fails, or can't be reached.

### POST /llm/embed

Ask a model for a vector for each of the messages, in the same order, with JSON, or for one message, with a `multipart/form-data` form, such as from a web browser. The JSON request has the `model`, the `messages`, each embedded by its `content`, which is its `text`, and its images and audio as `attachment`s, inline, with their `type` and `data`, and the `opt` as an object, such as `{"dimensions":256}`. The form has:

- `model`: the model, by name or as `provider:name`, such as `ollama:embeddinggemma-2:latest`, whose provider must embed messages.
- `text`: the text of the message.
- `file`: an image or audio of the message, uploaded as a file, for a model which embeds them, such as `embeddinggemma-2` on Ollama. An OpenAI-compatible provider embeds only text. The file's type is used, or detected from its data, or its name, when it's `application/octet-stream`.
- `opt`: the options, as a JSON object.

Returns a vector for each message, the `model` which embedded them, by its canonical name, and the tokens used. Vectors from different models can't be compared, so a model should be kept for the messages which are compared. Returns 400 if the request is invalid, has no messages or model, a message has no text or attachment, or the model's provider rejects it, such as for an option the model doesn't support, 404 if no provider has the model, 409 if more than one does, so its canonical name is needed, 413 if the request body is larger than 32 MiB, 501 if the model's provider can't embed messages, or the model is known not to, or 503 if the model couldn't embed the messages, such as when its provider fails, or can't be reached, or its own credentials are rejected.

## Event Operations

Events are changes to the registry's providers, which are streamed to a client as they happen. Each event has an `action`, which is `create` or `delete` when a provider is created or deleted, or `update` when whether it can be reached changes, when it's pinged, or its models change, when they're listed again, such as after a model is loaded or deleted. It also has the `table`, which is `provider`, and the `row`, which is the provider's name.

### GET /llm/event

Watch the registry's events, as a stream of server-sent events, until the client closes the connection. Each event is an `event`, whose data is the event, as JSON, such as `{"action":"create","table":"provider","row":"my-ollama"}`. The stream is watching once it has started, so a client which lists the providers then misses no changes. When the server is stopping, a `shutdown` event is sent, and the stream ends, as there will be no more events. A client which doesn't read its events has up to 16 kept for it, and misses those after them. Returns 406 if the client doesn't accept `text/event-stream`, or 503 if the server is stopping.
