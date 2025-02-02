# go-llm

Large Language Model API interface. This is a simple API interface for large language models
which run on [Ollama](https://github.com/ollama/ollama/blob/main/docs/api.md),
[Anthopic](https://docs.anthropic.com/en/api/getting-started) and [Mistral](https://docs.mistral.ai/).

The module includes the ability to utilize:

* Maintaining a session of messages
* Tool calling support
* Creating embeddings from text
* Streaming responses

There is a command-line tool included in the module which can be used to interact with the API.
For example,

```bash
# Display help
docker run ghcr.io/mutablelogic/go-llm:latest --help

# Interact with Claude to retrieve news headlines, assuming
# you have an API key for Anthropic and NewsAPI
docker run \
  --interactive -e ANTHROPIC_API_KEY -e NEWSAPI_KEY \
  ghcr.io/mutablelogic/go-llm:latest \
  chat claude-3-5-haiku-20241022
```

## Programmatic Usage

See the documentation [here](https://pkg.go.dev/github.com/mutablelogic/go-llm)
for integration into your own Go programs.

### Agent Instantiation

For each LLM provider, you create an agent which can be used to interact with the API.
To create an
[Ollama](https://pkg.go.dev/github.com/mutablelogic/go-llm/pkg/anthropic)
agent,

```go
import (
  "github.com/mutablelogic/go-llm/pkg/ollama"
)

func main() {
  // Create a new agent
  agent, err := ollama.New("https://ollama.com/api/v1/")
  if err != nil {
    panic(err)
  }
  // ...
}
```

To create an
[Anthropic](https://pkg.go.dev/github.com/mutablelogic/go-llm/pkg/anthropic)
agent,

```go
import (
  "github.com/mutablelogic/go-llm/pkg/anthropic"
)

func main() {
  // Create a new agent
  agent, err := anthropic.New(os.Getev("ANTHROPIC_API_KEY"))
  if err != nil {
    panic(err)
  }
  // ...
}
```

For Mistral models, you can use:

```go
import (
  "github.com/mutablelogic/go-llm/pkg/mistral"
)

func main() {
  // Create a new agent
  agent, err := mistral.New(os.Getev("MISTRAL_API_KEY"))
  if err != nil {
    panic(err)
  }
  // ...
}
```

### Chat Sessions

You create a **chat session** with a model as follows,

```go
import (
  "github.com/mutablelogic/go-llm"
)

func session(ctx context.Context, agent llm.Agent) error {
  // Create a new chat session
  session := agent.Model("claude-3-5-haiku-20241022").Context()

  // Repeat forever
  for {
    err := session.FromUser(ctx, "hello")
    if err != nil {
      return err
    }

    // Print the response
    fmt.Println(session.Text())
  }
}
```

## Options

You can add options to sessions, or to prompts. Different providers and models support
different options.

```go
type Model interface {
  // Set session-wide options
  Context(...Opt) Context

  // Add attachments (images, PDF's) to a user prompt
  UserPrompt(string, ...Opt) Context

  // Set embedding options
  Embedding(context.Context, string, ...Opt) ([]float64, error)
}

type Context interface {
  // Add single-use options when calling the model, which override
  // session options. You can also attach files to a user prompt.
  FromUser(context.Context, string, ...Opt) error
}
```

The options are as follows:

| Option | Ollama | Anthropic | Mistral | OpenAI | Description |
|--------|--------|-----------|---------|--------|-------------|
| `llm.WithTemperature(float64)` | Yes | Yes | Yes | - | What sampling temperature to use, between 0.0 and 1.0. Higher values like 0.7 will make the output more random, while lower values like 0.2 will make it more focused and deterministic. |
| `llm.WithTopP(float64)` | Yes | Yes | Yes | - | Nucleus sampling, where the model considers the results of the tokens with top_p probability mass. So 0.1 means only the tokens comprising the top 10% probability mass are considered. |
| `llm.WithTopK(uint64)` | Yes | Yes | No | - | Reduces the probability of generating nonsense. A higher value (e.g. 100) will give more diverse answers, while a lower value (e.g. 10) will be more conservative. |
| `llm.WithMaxTokens(uint64)` | - | Yes | Yes | - | The maximum number of tokens to generate in the response. |
| `llm.WithStream(func(llm.Completion))` | Can be enabled when tools are not used | Yes | Yes | - | Stream the response to a function. |
| `llm.WithToolChoice(string, string, ...)` | No | Yes | Use `auto`, `any`, `none`, `required` or a function name. Only the first argument is used. | - | The tool to use for the model. |
| `llm.WithToolKit(llm.ToolKit)` | Cannot be combined with streaming | Yes | Yes | - | The set of tools to use. |
| `llm.WithStopSequence(string, string, ...)` | Yes | Yes | Yes | - | Stop generation if one of these tokens is detected. |
| `llm.WithSystemPrompt(string)` | No | Yes | Yes | - | Set the system prompt for the model. |
| `llm.WithSeed(uint64)` | No | Yes | Yes | - | The seed to use for random sampling. If set, different calls will generate deterministic results. |
| `llm.WithFormat(string)` | No | Yes | Use `json_format` or `text` | - | The format of the response. For Mistral, you must also instruct the model to produce JSON yourself with a system or a user message. |
| `mistral.WithPresencePenalty(float64)` | No | No | Yes | - | Determines how much the model penalizes the repetition of words or phrases. A higher presence penalty encourages the model to use a wider variety of words and phrases, making the output more diverse and creative. |
| `mistral.WithFequencyPenalty(float64)` | No | No | Yes | - | Penalizes the repetition of words based on their frequency in the generated text. A higher frequency penalty discourages the model from repeating words that have already appeared frequently in the output, promoting diversity and reducing repetition. |
| `mistral.WithPrediction(string)` | No | No | Yes | - | Enable users to specify expected results, optimizing response times by leveraging known or predictable content. This approach is especially effective for updating text documents or code files with minimal changes, reducing latency while maintaining high-quality results. |
| `llm.WithSafePrompt()` | No | No | Yes | - | Whether to inject a safety prompt before all conversations. |
| `llm.WithNumCompletions(uint64)` | No | No | Yes | - | Number of completions to return for each request. |
| `llm.WithAttachment(io.Reader)` | Yes | Yes | Yes | - | Attach a file to a user prompt. It is the responsibility of the caller to close the reader. |
| `antropic.WithEphemeral()` | No | Yes | No | - | Attachments should be cached server-side |
| `antropic.WithCitations()` | No | Yes | No | - | Attachments should be used in citations |
| `antropic.WithUser(string)` | No | Yes | No | - | Indicate the user name for the request, for debugging |

## Contributing & Distribution

*This module is currently in development and subject to change*. Please do file
feature requests and bugs [here](https://github.com/mutablelogic/go-llm/issues).
The [license is Apache 2](LICENSE) so feel free to redistribute. Redistributions in either source
code or binary form must reproduce the copyright notice, and please link back to this
repository for more information:

> **go-llm**\
> [https://github.com/mutablelogic/go-llm/](https://github.com/mutablelogic/go-llm/)\
> Copyright (c) 2025 David Thorpe, All rights reserved.
