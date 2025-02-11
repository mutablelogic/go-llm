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
# Display version, help
docker run ghcr.io/mutablelogic/go-llm version
docker run ghcr.io/mutablelogic/go-llm --help

# Interact with Claude to retrieve news headlines, assuming
# you have an API key for both Anthropic and NewsAPI
docker run -e ANTHROPIC_API_KEY -e NEWSAPI_KEY \
  ghcr.io/mutablelogic/go-llm \
  chat mistral-small-latest --prompt "What is the latest news?"
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

For example, if you want to implement a tool which adds two numbers,

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
  toolkit.Register(&Adder{})

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

Parameters are implemented as struct fields, with tags. The tags you can include are:

* `name:""` - Set the name for the parameter
* `json:""` - If `name` is not used, then the name is set from the `json` tag
* `help:":` - Set the description for the parameter
* `required:""` - The parameter is required as part of the tool call
* `enum:"a,b,c"` - The parameter value should be one of these comma-separated options

The transation of field types is as follows:

* `string` - Translates as JSON `string`
* `uint`, `int` - Translates to JSON `integer`
* `float32`, `float64` - Translates to JSON `number`

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
  -v, --verbose                   Enable verbose output
      --timeout=DURATION          Agent connection timeout
      --ollama-endpoint=STRING    Ollama endpoint ($OLLAMA_URL)
      --anthropic-key=STRING      Anthropic API Key ($ANTHROPIC_API_KEY)
      --mistral-key=STRING        Mistral API Key ($MISTRAL_API_KEY)
      --open-ai-key=STRING        OpenAI API Key ($OPENAI_API_KEY)
      --gemini-key=STRING         Gemini API Key ($GEMINI_API_KEY)
      --news-key=STRING           News API Key ($NEWSAPI_KEY)

Commands:
  agents       Return a list of agents
  models       Return a list of models
  tools        Return a list of tools
  download     Download a model (for Ollama)
  chat         Start a chat session
  complete     Complete a prompt, generate image or speech from text
  embedding    Generate an embedding
  version      Print the version of this tool

Run "llm <command> --help" for more information on a command.
```

### Prompt Completion

To have the model respond to a prompt, you can use the `complete` command. For example, to
have the model respond to the prompt "What is the capital of France?" using the `claude-3-5-haiku-20241022`
model, you can use the following command:

```bash
llm complete "What is the capital of France?"
```

The first time you use the command use the ``--model`` flag to specify the model you want to use. Your
choice of model will be remembered for subsequent completions.

### Explain computer code

To have the model explain a piece of computer code, you can pipe the code into the `complete` command.
For example, to have the model explain the code in the file `example.go`, you can use the following command:

```bash
cat example.go | llm complete
```

### Caption an image

To have the model generate a caption for an image, you can use the `complete` command with the `--file`
flag. For example, to have the model generate a caption for the image in the file `example.jpg`, you can use
the following command:

```bash
llm complete --model gpt-4o --file picture.png "Explain this image"
```

### Generate an image

To have the model generate an image from a prompt, you can use the `complete` command with the `--format image`
option. For example, to have the model generate an image from the prompt "A picture of a cat", you can use
the following command:

```bash
llm complete --model dall-e-3 --format image "A picture of a cat"
```

It will write the file in the current working directory.

## Contributing & Distribution

_This module is currently in development and subject to change_. Please do file
feature requests and bugs [here](https://github.com/mutablelogic/go-llm/issues).
The [license is Apache 2](LICENSE) so feel free to redistribute. Redistributions in either source
code or binary form must reproduce the copyright notice, and please link back to this
repository for more information:

> **go-llm**\
> [https://github.com/mutablelogic/go-llm/](https://github.com/mutablelogic/go-llm/)\
> Copyright (c) 2025 David Thorpe, All rights reserved.
