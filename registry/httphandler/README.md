# LLM Registry

## Provider Operations

Providers are LLM backends, such as Ollama, in the registry. Each has a name in the registry, unique across its providers, and a type, so there can be more than one provider of a type, such as two Ollama servers. A model is referred to by its name, or by its provider's name and its name, separated by a colon, such as `my-ollama:llama3.2:latest`.

The types are:

- `ollama`: an Ollama server. A `url` without a scheme, such as `localhost`, uses `http` and Ollama's port, 11434.
- `openai`: OpenAI, which is the default `url`, or a server with an OpenAI-compatible API, such as DeepSeek, vLLM or LM Studio.
- `eliza`: a mock provider for testing, which has no `url` or API key.

The registry isn't stored, so providers which are created are lost when the server stops, and an API key is sent as a bearer token, and is never returned. An API key can be a credential's placeholder, such as `${OPENAI_KEY}`, which is replaced with the credential, which is the environment variable with that name, unless the server has another credentials provider. A provider is pinged when it's listed or got, and has an `error` when it can't be reached.

### GET /llm/provider

List the providers, in order of their names, with what each can do, such as generate messages, or download models, and an `error` for each which can't be reached. The `provider` parameter returns only the providers of that type, such as `ollama`. Returns 400 if a parameter is invalid.

### POST /llm/provider

Create a provider of a type, with its `name` in the registry, which is an identifier, of up to 64 letters, numbers, underscores or hyphens, starting with a letter, or its type when it's omitted, and its `url` and `api_key`, which aren't needed for every type. The provider is checked to be reachable before it's added. Returns the created provider. Returns 400 if the request body is invalid, or the type is unknown, 409 if a provider with that name exists already, or 503 if the provider can't be reached.

### GET /llm/provider/{name}

Get a provider by name. Returns 404 if there is no provider with that name.

### DELETE /llm/provider/{name}

Delete a provider by name, from the registry. Its models can't be used afterwards. Returns the deleted provider. Returns 404 if there is no provider with that name.
