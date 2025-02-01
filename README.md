# go-llm

Large Language Model API interface. This is a simple API interface for large language models
which run on [Ollama](https://github.com/ollama/ollama/blob/main/docs/api.md)
and [Anthopic](https://docs.anthropic.com/en/api/getting-started).

The module includes the ability to utilize:

* Maintaining a session of messages
* Tool calling support
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
for integration into your own Go programs. To create an
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

## Contributing & Distribution

*This module is currently in development and subject to change*. Please do file
feature requests and bugs [here](https://github.com/mutablelogic/go-llm/issues).
The [license is Apache 2](LICENSE) so feel free to redistribute. Redistributions in either source
code or binary form must reproduce the copyright notice, and please link back to this
repository for more information:

> **go-llm**\
> [https://github.com/mutablelogic/go-llm/](https://github.com/mutablelogic/go-llm/)\
> Copyright (c) 2025 David Thorpe, All rights reserved.
