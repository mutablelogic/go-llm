package agent

import (
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Opt is a functional option for configuring an agent
type Opt func(*agent) error

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithClient adds an LLM client to the agent
func WithClient(client llm.Client) Opt {
	return func(a *agent) error {
		a.clients[client.Name()] = client
		return nil
	}
}
