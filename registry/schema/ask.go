package schema

import (
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AskRequest asks a model for its reply to the messages, which include
// earlier turns of the conversation, after the system prompt, with the
// options. The model can call the tools, whose calls are in its reply, for
// the caller to run, and send their results back. With a schema, the model
// replies with JSON which conforms to it.
type AskRequest struct {
	GeneratorMeta
	Messages []providerschema.MessageMeta `json:"messages" help:"Messages, including earlier turns of the conversation, and the results of the tools the model called"`
}

// AskResponse is the model's reply, with its canonical name, its result,
// when the request has a schema, and the model replied with JSON which
// conforms to it, and the tokens used
type AskResponse struct {
	Model   string                 `json:"model" help:"Canonical name of the model which replied" example:"ollama:llama3.2:latest"`
	Message providerschema.Message `json:"message" help:"The model's reply, which can have the tools it called"`
	Schema  bool                   `json:"schema,omitempty" help:"Whether the request has a schema, when the reply's result is missing when the model didn't reply with it"`
	Result  json.RawMessage        `json:"result,omitempty" help:"The reply, as JSON which conforms to the request's schema, when it has one, and the model replied with it"`
	Usage   llm.Usage              `json:"usage" help:"Tokens used, by every request to the model"`
}

// StreamEvent is the data of a text or thinking event, which is a piece of
// the reply as it's generated
type StreamEvent struct {
	Text string `json:"text" help:"A piece of the reply's text, or of the model's thinking"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The events of a stream of a model's reply: a text or thinking event for
// each piece of the reply as it's generated, a log event for each line a
// tool logs as it runs, a progress event, as a model's download is, of how
// a tool is getting on, and then a result event, with the response, or an
// error event
const (
	EventText     = string(llm.StreamText)
	EventThinking = string(llm.StreamThinking)
	EventLog      = string(llm.StreamLog)
)

// AskBodyLimit is the largest request body, in bytes, when a model is asked,
// which allows for images in the messages
const AskBodyLimit = 32 << 20

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r AskRequest) String() string {
	return types.Stringify(r)
}

func (r AskResponse) String() string {
	return types.Stringify(r)
}
