package schema

import (
	"bytes"
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
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
	Model    string                       `json:"model" help:"Model, by name, or by provider and name" example:"ollama:llama3.2:latest"`
	System   string                       `json:"system,omitempty" help:"System prompt, sent before the messages" example:"Be brief" optional:""`
	Tools    []string                     `json:"tools,omitzero" help:"Tools the model can call, by name, by namespace and name, all the tools in a namespace, such as weatherapi:*, or * for all the tools" example:"[\"weatherapi:current\"]" optional:""`
	Opt      map[string]json.RawMessage   `json:"opt,omitzero" help:"Options, by key, such as temperature=0.7, which the provider decodes" example:"{\"temperature\":0.7}" optional:""`
	Schema   json.RawMessage              `json:"schema,omitempty" help:"JSON schema of the reply, which the model replies with, or text when omitted" example:"{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"}}}" optional:""`
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
// each piece of the reply as it's generated, and then a result event, with
// the response, or an error event
const (
	EventText     = string(llm.StreamText)
	EventThinking = string(llm.StreamThinking)
)

// AskBodyLimit is the largest request body, in bytes, when a model is asked,
// which allows for images in the messages
const AskBodyLimit = 32 << 20

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// JSONSchema returns the schema of the reply, or nil when the reply is text.
// Returns ErrBadParameter when it isn't a JSON schema.
func (r AskRequest) JSONSchema() (*jsonschema.Schema, error) {
	data := bytes.TrimSpace(r.Schema)
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	result, err := jsonschema.FromJSON(json.RawMessage(data))
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid schema: %v", err)
	}
	return result, nil
}

// Opts returns the system prompt, and the options, for the provider to
// decode. Returns ErrBadParameter when an option is one which the request
// has a field for, which is the system prompt, and the schema.
func (r AskRequest) Opts(system string) ([]llm.Opt, error) {
	opts := make([]llm.Opt, 0, len(r.Opt)+1)
	if system = strings.TrimSpace(system); system != "" {
		opts = append(opts, llm.WithSystemPrompt(system))
	}
	for key, value := range r.Opt {
		switch strings.TrimSpace(key) {
		case llm.SystemPromptKey:
			return nil, llm.ErrBadParameter.Withf("invalid option %q: set the system prompt as system", key)
		case llm.JSONSchemaKey:
			return nil, llm.ErrBadParameter.Withf("invalid option %q: set the schema of the reply as schema", key)
		}
		opts = append(opts, llm.WithOpt(key, value))
	}
	return opts, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r AskRequest) String() string {
	return types.Stringify(r)
}

func (r AskResponse) String() string {
	return types.Stringify(r)
}
