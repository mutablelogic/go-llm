package schema

import (

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SessionMeta is a session's settings: how its model is asked
type SessionMeta struct {
	GeneratorMeta
}

// Session is a conversation with a model: its settings, and its messages,
// each with its tokens, as they're kept
type Session struct {
	SessionMeta
	Messages []providerschema.Message `json:"messages,omitzero" help:"Messages of the conversation, each with its tokens"`
}

// SessionResponse is what's added to a session's conversation: the
// messages, the last of which is the model's reply, each with its tokens,
// and the tokens of the conversation, with them, of the messages sent to
// the model, of its replies, and of what's sent with them, such as the
// system prompt and the tools, which together are about the tokens of the
// next request, and the usage of every request to the model. With a schema,
// the model's reply, when its result is stop, is JSON which conforms to it.
type SessionResponse struct {
	Messages []providerschema.Message `json:"messages" help:"Messages to add to the conversation: those sent, and those added, the last of which is the model's reply, each with its tokens"`
	Input    uint                     `json:"input" help:"Tokens of the conversation's messages sent to the model, such as from the user, and the results of tools" example:"1200"`
	Output   uint                     `json:"output" help:"Tokens of the conversation's replies from the model" example:"300"`
	Overhead uint                     `json:"overhead" help:"Tokens of what's sent with the messages, such as the system prompt and the tools" example:"150"`
	Usage    llm.Usage                `json:"usage" help:"Tokens used, by every request to the model"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (s SessionMeta) String() string {
	return types.Stringify(s)
}

func (s Session) String() string {
	return types.Stringify(s)
}

func (s SessionResponse) String() string {
	return types.Stringify(s)
}
