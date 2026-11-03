# LLM Registry

## Provider Operations

Providers are LLM backends, such as Ollama, in the registry. Each has a name in the registry, unique across its providers, and a type, so there can be more than one provider of a type, such as two Ollama servers. A model is referred to by its name, or by its provider's name and its name, separated by a colon, such as `my-ollama:llama3.2:latest`.

The types are:

- `ollama`: an Ollama server. A `url` without a scheme, such as `localhost`, uses `http` and Ollama's port, 11434.
- `openai`: OpenAI, which is the default `url`, or a server with an OpenAI-compatible API, such as DeepSeek, vLLM or LM Studio.
- `eliza`: a mock provider for testing, which has no `url` or API key.

The registry isn't stored, so providers which are created are lost when the server stops, and an API key is sent as a bearer token, and is never returned. An API key can be a credential's placeholder, such as `${OPENAI_KEY}`, which is replaced with the credential, which is the environment variable with that name, unless the server has another credentials provider. A provider is pinged when it's listed or got, unless it was pinged within the last minute, and has an `error` when it can't be reached. A provider's models are those whose names match one of its `include` patterns, or all of them when it has none, and none of its `exclude` patterns, which are regular expressions, such as `^gpt-`. They're cached, and listed again after six hours, or every minute for a provider which downloads models, such as Ollama, or when they couldn't be listed.

### GET /llm/provider

List the providers, in order of their names, with what each can do, such as generate messages, or download models, and an `error` for each which can't be reached. The `provider` parameter returns only the providers of that type, such as `ollama`. Returns 400 if a parameter is invalid.

### POST /llm/provider

Create a provider of a type, with its `name` in the registry, which is an identifier, of up to 64 letters, numbers, underscores or hyphens, starting with a letter, or its type when it's omitted, and its `url` and `api_key`, which aren't needed for every type, and the `include` and `exclude` patterns of the names of its models. The provider is checked to be reachable before it's added. Returns the created provider. Returns 400 if the request body is invalid, the type is unknown, or a pattern isn't a valid regular expression, 409 if a provider with that name exists already, or 503 if the provider can't be reached.

### GET /llm/provider/{name}

Get a provider by name. Returns 404 if there is no provider with that name.

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
