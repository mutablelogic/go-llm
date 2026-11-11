# Tools and Agents

## Tools

Tools are what models call, in namespaces, such as `weatherapi:current`. A conversation is given tools by name, by namespace (`weatherapi:*`), or all of them (`*`).

| Namespace | Tools | Needs |
|-----------|-------|-------|
| `memory` | Remember details about the person and the assistant; search, keep and forget memories | Always added |
| `heartbeat` | Schedule, list, update and cancel tasks | Always added |
| `llm` | List models, speak, make images, read documents, transcribe, read the conversation's files, and manage agents | Always added |
| `weatherapi` | Current weather, forecasts and alerts | `WEATHERAPI_API_KEY` |
| `newsapi` | News articles, headlines and sources | `NEWSAPI_API_KEY` |
| `homeassistant` | Query, search and control smart home devices | `HOMEASSISTANT_URL`, `HOMEASSISTANT_API_KEY` |
| `ollama` | Web search and fetch, with Ollama's cloud | `OLLAMA_API_KEY` |
| `deepseek` | Web search and fetch, with DeepSeek's | `DEEPSEEK_API_KEY` |
| `messaging` | Connect, list, send to and disconnect chats, such as on Telegram | `TELEGRAM_BOT_TOKEN` |
| *a connector's* | The tools of a remote MCP server | A connector |

- **Web search costs money:** Ollama's is free up to a monthly quota, and DeepSeek's is charged per call.
- **Browsing them:** the Tools page lists the namespaces and their tools, or `llm tools`, and `llm call` calls one.

### The `llm` tools

- **`llm:models`** lists models by capability, and, with `speech_output` and a model, its voices, a page at a time.
- **`llm:speak`** speaks text, in a voice, by its ID or name. A voice note, for Telegram, is Ogg Opus.
- **`llm:transcribe`, `llm:ocr` and `llm:illustrate`** transcribe speech, read documents, and make images.
- **`llm:resource`** lists and reads the conversation's files. See [Workspaces](workspaces.md).
- **`llm:agent`** lists, gets, creates and updates agents.

## Connectors

Connectors are remote MCP servers, whose tools are a namespace of their own, such as `linear:list_issues`.

- **Adding one:** **Add connector** on the Tools page lists well-known servers, such as GitHub, Linear, Notion and Stripe, and **Custom connector** for any URL. Or `llm connector-create <url>`.
- **Signing in:** a server which needs it has a **Sign in** button, or the command line opens your browser. go-llm keeps the token, and refreshes it.
- **Your own OAuth app:** some servers, such as GitHub's, need your app's ID and secret, in the server's environment, such as `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`.
- **Prompts are agents:** a server's MCP prompts are agents in its namespace, such as `github:review_pr`.
- **Behind a reverse proxy:** run the server with `--tls.name=<public host>`, so sign-in returns to the public address.

## Agents

An agent is a prompt with its own model, tools, and input and reply schemas, which models call as a tool.

- **Yours** are in the `agent` namespace, such as `agent:segment`, kept as YAML or JSON. Add them on the Tools page, with `llm agent-create`, or ask the assistant, with `llm:agent`.
- **Built-in ones** are in the `builtin` namespace, such as `builtin:translate`. Only their model can be changed.
- **Each call is new:** an agent doesn't see the conversation, or its earlier calls, so it's given what it needs in its input.
- **Nesting:** agents can call agents, as deep as `max_depth` allows.

## The agent loop

These options control how long a model, or an agent, works before it must reply:

| Option | Default | Meaning |
|--------|---------|---------|
| `max_iterations` | 10, or 3 for `ask` | How many times the model is asked before it must reply |
| `max_depth` | 3 | How deeply agents can call agents |
| `tool_timeout` | `30s` | How long a tool has to return |
