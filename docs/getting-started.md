# Getting Started

Once go-llm is running, and the command line is installed (see [Installation](installation.md)), add providers, and start a conversation.

## Add providers

Add providers in the web UI, with **Add provider** on the Models page, or with the command line:

```bash
llm provider-create-ollama http://host.docker.internal:11434
llm provider-create-lmstudio http://host.docker.internal:1234/v1
llm provider-create-openai --api-key-credential OPENAI_API_KEY
llm provider-create-anthropic --api-key-credential ANTHROPIC_API_KEY
llm provider-create-gemini --api-key-credential GEMINI_API_KEY

llm providers
llm models
```

- **API keys** are given by the name of the server's environment variable, so the key itself is never stored or sent.
- **URLs** are as the server sees them, rather than the machine running `llm`.

See [Models and Providers](models-and-providers.md) for every provider.

## Ask, and chat

```bash
llm ask --model gpt-4o-mini "Explain a KV cache in two sentences"
llm ask --attach main.go "What does this do?"

llm session-create --model ollama:llama3.2 --system "Be brief" --tools "weatherapi:*"
llm chat                                        # in the terminal
llm chat "Will it rain in Berlin tomorrow?"     # once
```

Or chat in the web UI, with **Add conversation**. See [Conversations and Tasks](conversations-and-tasks.md).

## Commands

| Group | Commands |
|-------|----------|
| Server | `run`, `openapi` |
| Providers | `providers`, `provider`, `provider-create-ollama`, `provider-create-lmstudio`, `provider-create-openai`, `provider-create-anthropic`, `provider-create-gemini`, `provider-create-mistral`, `provider-create-deepseek`, `provider-create-gowhisper`, `provider-create-elevenlabs`, `provider-create-eliza`, `provider-update`, `provider-delete` |
| Models | `models`, `model`, `model-load`, `model-delete`, `model-unload`, `model-voices` |
| Sessions | `sessions`, `session`, `session-messages`, `session-create`, `session-update`, `session-forget`, `session-compact`, `session-delete` |
| Inference | `ask`, `chat`, `embed`, `ocr`, `transcribe`, `translate`, `speak`, `illustrate` |
| Tools and agents | `namespaces`, `tools`, `tool`, `call`, `agents`, `agent`, `agent-create`, `agent-update`, `agent-delete` |
| Connectors | `connectors`, `connector`, `connector-create`, `connector-update`, `connector-delete` |
| Memory | `details`, `detail-set`, `detail-forget`, `memories`, `memory`, `memory-remember`, `memory-forget` |
| Heartbeats | `heartbeats`, `heartbeat`, `heartbeat-create`, `heartbeat-update`, `heartbeat-delete` |
| Resources | `resources`, `resource`, `resource-create`, `resource-get`, `resource-delete` |
| Events | `events` |

Run `llm <command> --help` for its flags. Model options are given with `--opt`, such as `--opt temperature=0.2`. These options control the agent loop rather than the model:

| Option | Default | Meaning |
|--------|---------|---------|
| `max_iterations` | 10, or 3 for `ask` | How many times the model is asked before it must reply |
| `max_depth` | 3 | How deeply agents can call agents |
| `tool_timeout` | `30s` | How long a tool has to return |
