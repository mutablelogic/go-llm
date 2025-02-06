# go-llm

The module implements a simple API interface for large language models
which run on [Ollama](https://github.com/ollama/ollama/blob/main/docs/api.md),
[Anthopic](https://docs.anthropic.com/en/api/getting-started), [Mistral](https://docs.mistral.ai/)
and [OpenAI](https://platform.openai.com/docs/api-reference). The module implements the ability to:

* Maintain a session of messages;
* Tool calling support, including using your own tools (aka Tool plugins);
* Create embedding vectors from text;
* Stream responses;
* Multi-modal support (aka, Images, Audio and Attachments);
* Text-to-speech (OpenAI only) for completions

There is a command-line tool included in the module which can be used to interact with the API.
If you have docker installed, you can use the following command to run the tool, without
installation:

```bash
# Display help
docker run ghcr.io/mutablelogic/go-llm:latest --help

# Interact with Claude to retrieve news headlines, assuming
# you have an API key for Anthropic and NewsAPI
docker run \
  -e OLLAMA_URL -e MISTRAL_API_KEY -e ANTHROPIC_API_KEY -e OPENAI_API_KEY \
  -e NEWSAPI_KEY \
  ghcr.io/mutablelogic/go-llm:latest \
  chat mistral-small-latest --prompt "What is the latest news?" --no-stream
```

See below for more information on how to use the command-line tool (or how to
install it if you have a `go` compiler).

## Programmatic Usage

See the documentation [here](https://pkg.go.dev/github.com/mutablelogic/go-llm)
for integration into your own code.

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
  // Create a new agent - replace the URL with the one to your Ollama instance
  agent, err := ollama.New("https://ollama.com/api/v1/")
  if err != nil {
    panic(err)
  }
  // ...
}
```

To create an
[Anthropic](https://pkg.go.dev/github.com/mutablelogic/go-llm/pkg/anthropic)
agent with an API key stored as an environment variable,

```go
import (
  "github.com/mutablelogic/go-llm/pkg/anthropic"
)

func main() {
  // Create a new agent
  agent, err := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
  if err != nil {
    panic(err)
  }
  // ...
}
```

For [Mistral](https://pkg.go.dev/github.com/mutablelogic/go-llm/pkg/mistral) models, you can use:

```go
import (
  "github.com/mutablelogic/go-llm/pkg/mistral"
)

func main() {
  // Create a new agent
  agent, err := mistral.New(os.Getenv("MISTRAL_API_KEY"))
  if err != nil {
    panic(err)
  }
  // ...
}
```

Similarly for [OpenAI](https://pkg.go.dev/github.com/mutablelogic/go-llm/pkg/openai)
models, you can use:

```go
import (
  "github.com/mutablelogic/go-llm/pkg/openai"
)

func main() {
  // Create a new agent
  agent, err := openai.New(os.Getenv("OPENAI_API_KEY"))
  if err != nil {
    panic(err)
  }
  // ...
}
```

You can append options to the agent creation to set the client/server communication options,
such as user agent strings, timeouts, debugging, rate limiting, adding custom headers, etc. See [here](https://pkg.go.dev/github.com/mutablelogic/go-client#readme-basic-usage) for more information.

There is also an _aggregated_ agent which can be used to interact with multiple providers at once. This is useful if you want
to use models from different providers simultaneously.

```go
import (
  "github.com/mutablelogic/go-llm/pkg/agent"
)

func main() {
  // Create a new agent which aggregates multiple providers
  agent, err := agent.New(
    agent.WithAnthropic(os.Getenv("ANTHROPIC_API_KEY")), 
    agent.WithMistral(os.Getenv("MISTRAL_API_KEY")),
    agent.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
    agent.WithOllama(os.Getenv("OLLAMA_URL")),
  )
  if err != nil {
    panic(err)
  }
  // ...
}
```

### Completion

You can generate a **completion** as follows,

```go
import (
  "github.com/mutablelogic/go-llm"
)

func completion(ctx context.Context, agent llm.Agent) (string, error) {
  completion, err := agent.
    Model(ctx, "claude-3-5-haiku-20241022").
    Completion((ctx, "Why is the sky blue?")
  if err != nil {
    return "", err
  } else {
    return completion.Text(0), nil
  }
}
```

The zero index argument on `completion.Text(int)` indicates you want the text from the zero'th completion
choice, for providers who can generate serveral different choices simultaneously.

### Chat Sessions

You create a **chat session** with a model as follows,

```go
import (
  "github.com/mutablelogic/go-llm"
)

func session(ctx context.Context, agent llm.Agent) error {
  // Create a new chat session
  session := agent.
    Model(ctx, "claude-3-5-haiku-20241022").
    Context()

  // Repeat forever
  for {
    err := session.FromUser(ctx, "hello")
    if err != nil {
      return err
    }

    // Print the response for the zero'th completion
    fmt.Println(session.Text(0))
  }
}
```

The `Context` object will continue to store the current session and options, and will
ensure the session is maintained across multiple completion calls.

### Embedding Generation

You can generate embedding vectors using an appropriate model with Ollama, OpenAI and Mistral models:

```go
import (
  "github.com/mutablelogic/go-llm"
)

func embedding(ctx context.Context, agent llm.Agent) error {
  vector, err := agent.
    Model(ctx, "mistral-embed").
    Embedding(ctx, "hello")
  // ...
}
```

### Attachments & Image Caption Generation

Some models have `vision` capability and others can also summarize text. For example, to
generate captions for an image,

```go
import (
  "github.com/mutablelogic/go-llm"
)

func generate_image_caption(ctx context.Context, agent llm.Agent, path string) (string, error) {
  f, err := os.Open(path)
  if err != nil {
    return "", err
  }
  defer f.Close()

  completion, err := agent.
    Model(ctx, "claude-3-5-sonnet-20241022").
    Completion((ctx, "Provide a short caption for this image", llm.WithAttachment(f))
  if err != nil {
    return "", err  
  }    

  return completion.Text(0), nil
}
```

To summarize a text or PDF document is exactly the same using an Anthropic model, but maybe
with a different prompt.

### Streaming

Streaming is supported with all providers, but Ollama cannot be used with streaming and tools
simultaneously. You provide a callback function of signature `func(llm.Completion)` which will
be called as a completion is received.

```go
import (
  "github.com/mutablelogic/go-llm"
)

func generate_completion(ctx context.Context, agent llm.Agent, prompt string) (string, error) {
   completion, err := agent.
    Model(ctx, "claude-3-5-haiku-20241022").
    Completion((ctx, "Why is the sky blue?", llm.WithStream(stream_callback))
  if err != nil {
    return "", err
  } else {
    return completion.Text(0), nil
  }
}

func stream_callback(completion llm.Completion) {
  // Print out the completion text on each call
  fmt.Println(completion.Text(0))
}

```

### Tool Support

All providers support tools, but not all models. Your own tools should implement the
following interface:

```go
package llm

// Definition of a tool
type Tool interface {
  Name() string                     // The name of the tool
  Description() string              // The description of the tool
  Run(context.Context) (any, error) // Run the tool with a deadline and 
                                    // return the result
}
```

For example, if you want to implement a tool which adds to numbers,

```go
package addition

type Adder struct {
  A float64 `name:"a" help:"The first number" required:"true"`
  B float64 `name:"b" help:"The second number" required:"true"`
}

func (Adder) Name() string {
  return "add_two_numbers"
}

func (Adder) Description() string {
  return "Add two numbers together and return the result"
}

func (a Adder) Run(context.Context) (any, error) {
  return a.A + a.B, nil
}
```

Then you can include your tool as part of the completion. It's possible that a
completion will continue to call additional tools, in which case you should
actually loop through completions until no tool calls are made.

```go
import (
  "github.com/mutablelogic/go-llm"
  "github.com/mutablelogic/go-llm/pkg/tool"
)

func add_two_numbers(ctx context.Context, agent llm.Agent) (string, error) {
  context := agent.Model(ctx, "claude-3-5-haiku-20241022").Context()
  toolkit := tool.NewToolKit()
  toolkit.Register(Adder{})

  // Get the tool call
  if err := context.FromUser(ctx, "What is five plus seven?", llm.WithToolKit(toolkit)); err != nil {
    return "", err
  }

  // Call tools
  for {
    calls := context.ToolCalls(0)
    if len(calls) == 0 {
      break
    }

    // Print out any intermediate messages
    if context.Text(0) != "" {
      fmt.Println(context.Text(0))      
    }

    // Get the results from the toolkit
    results, err := toolkit.Run(ctx, calls...)
    if err != nil {
      return "", err
    }

    // Get another tool call or a user response
    if err := context.FromTool(ctx, results...); err != nil {
      return "", err
    }
  }

  // Return the result
  return context.Text(0)
}
```

## Options

You can add options to sessions, or to prompts. Different providers and models support
different options.

```go
package llm 

type Model interface {
  // Set session-wide options
  Context(...Opt) Context

  // Create a completion from a text prompt
  Completion(context.Context, string, ...Opt) (Completion, error)

  // Create a completion from a chat session
  Chat(context.Context, []Completion, ...Opt) (Completion, error)

  // Embedding vector generation
  Embedding(context.Context, string, ...Opt) ([]float64, error)
}

type Context interface {
  // Generate a response from a user prompt (with attachments and
  // other options)
  FromUser(context.Context, string, ...Opt) error
}
```

The options are as follows:

| Option | Ollama | Anthropic | Mistral | OpenAI | Description |
|--------|--------|-----------|---------|--------|-------------|
| `llm.WithTemperature(float64)` | Yes | Yes | Yes | Yes | What sampling temperature to use, between 0.0 and 1.0. Higher values like 0.7 will make the output more random, while lower values like 0.2 will make it more focused and deterministic. |
| `llm.WithTopP(float64)` | Yes | Yes | Yes | Yes | Nucleus sampling, where the model considers the results of the tokens with top_p probability mass. So 0.1 means only the tokens comprising the top 10% probability mass are considered. |
| `llm.WithTopK(uint64)` | Yes | Yes | No | No | Reduces the probability of generating nonsense. A higher value (e.g. 100) will give more diverse answers, while a lower value (e.g. 10) will be more conservative. |
| `llm.WithMaxTokens(uint64)` | No | Yes | Yes | Yes | The maximum number of tokens to generate in the response. |
| `llm.WithStream(func(llm.Completion))` | Can be enabled when tools are not used | Yes | Yes | Yes | Stream the response to a function. |
| `llm.WithToolChoice(string, string, ...)` | No | Use `auto`, `any` or a function name. Only the first argument is used. | Use `auto`, `any`, `none`, `required` or a function name. Only the first argument is used. | Use `auto`, `none`, `required` or a function name. Only the first argument is used. | The tool to use for the model. |
| `llm.WithToolKit(llm.ToolKit)` | Cannot be combined with streaming | Yes | Yes | Yes | The set of tools to use. |
| `llm.WithStopSequence(string, string, ...)` | Yes | Yes | Yes | Yes | Stop generation if one of these tokens is detected. |
| `llm.WithSystemPrompt(string)` | No | Yes | Yes | Yes | Set the system prompt for the model. |
| `llm.WithSeed(uint64)` | Yes | No | Yes | Yes | The seed to use for random sampling. If set, different calls will generate deterministic results. |
| `llm.WithFormat(string)` | Use `json` | No | Use `json_format` or `text` | Use `json_format` or `text` | The format of the response. For Mistral, you must also instruct the model to produce JSON yourself with a system or a user message. |
| `llm.WithPresencePenalty(float64)` | Yes | No | Yes | Yes | Determines how much the model penalizes the repetition of words or phrases. A higher presence penalty encourages the model to use a wider variety of words and phrases, making the output more diverse and creative. |
| `llm.WithFequencyPenalty(float64)` | Yes | No | Yes | Yes | Penalizes the repetition of words based on their frequency in the generated text. A higher frequency penalty discourages the model from repeating words that have already appeared frequently in the output, promoting diversity and reducing repetition. |
| `llm.WithPrediction(string)` | No | No | Yes | Yes | Enable users to specify expected results, optimizing response times by leveraging known or predictable content. This approach is especially effective for updating text documents or code files with minimal changes, reducing latency while maintaining high-quality results. |
| `llm.WithSafePrompt()` | No | No | Yes | No | Whether to inject a safety prompt before all conversations. |
| `llm.WithNumCompletions(uint64)` | No | No | Yes | Yes | Number of completions to return for each request. |
| `llm.WithAttachment(io.Reader)` | Yes | Yes | Yes | - | Attach a file to a user prompt. It is the responsibility of the caller to close the reader. |
| `llm.WithUser(string)` | No | Yes | No | Yes | A unique identifier representing your end-user |
| `antropic.WithEphemeral()` | No | Yes | No | - | Attachments should be cached server-side |
| `antropic.WithCitations()` | No | Yes | No | - | Attachments should be used in citations |
| `openai.WithStore(bool)` | No | No | No | Yes | Whether or not to store the output of this chat completion request |
| `openai.WithDimensions(uint64)` | No | No | No | Yes | The number of dimensions the resulting output embeddings should have. Only supported in text-embedding-3 and later models |
| `openai.WithReasoningEffort(string)` | No | No | No | Yes | The level of effort model should put into reasoning. |
| `openai.WithMetadata(string, string)` | No | No | No | Yes | Metadata to be logged with the completion. |
| `openai.WithLogitBias(uint64, int64)` | No | No | No | Yes | A token and their logit bias value. Call multiple times to add additional tokens |
| `openai.WithLogProbs()` | No | No | No | Yes | Include the log probabilities on the completion. |
| `openai.WithLogProbs()` | No | No | No | Yes | Include the log probabilities on the completion. |
| `openai.WithTopLogProbs(uint64)` | No | No | No | Yes | An integer between 0 and 20 specifying the number of most likely tokens to return at each token position. |
| `openai.WithAudio(string, string)` | No | No | No | Yes | Output audio (voice, format) for the completion. Can be used with certain models. |
| `openai.WithServiceTier(string)` | No | No | No | Yes | Specifies the latency tier to use for processing the request. |
| `openai.WithStreamOptions(func(llm.Completion), bool)` | No | No | No | Yes | Include usage information in the stream response |
| `openai.WithDisableParallelToolCalls()` | No | No | No | Yes | Call tools in serial, rather than in parallel |

## The Command Line Tool

You can use the command-line tool to interact with the API. To build the tool, you can use the following command:

```bash
go install github.com/mutablelogic/go-llm/cmd/llm@latest
llm --help
```

The output is something like:

```text
Usage: llm <command> [flags]

LLM agent command line interface

Flags:
  -h, --help                      Show context-sensitive help.
      --debug                     Enable debug output
      --verbose                   Enable verbose output
      --ollama-endpoint=STRING    Ollama endpoint ($OLLAMA_URL)
      --anthropic-key=STRING      Anthropic API Key ($ANTHROPIC_API_KEY)
      --news-key=STRING           News API Key ($NEWSAPI_KEY)

Commands:
  agents      Return a list of agents
  models      Return a list of models
  tools       Return a list of tools
  download    Download a model
  chat        Start a chat session

Run "llm <command> --help" for more information on a command.
```

## Contributing & Distribution

_This module is currently in development and subject to change_. Please do file
feature requests and bugs [here](https://github.com/mutablelogic/go-llm/issues).
The [license is Apache 2](LICENSE) so feel free to redistribute. Redistributions in either source
code or binary form must reproduce the copyright notice, and please link back to this
repository for more information:

> **go-llm**\
> [https://github.com/mutablelogic/go-llm/](https://github.com/mutablelogic/go-llm/)\
> Copyright (c) 2025 David Thorpe, All rights reserved.
