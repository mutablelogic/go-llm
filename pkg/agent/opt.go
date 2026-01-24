package agent

import (
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
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

// WithProvider is used by ListModels to choose which provider to list models from
func WithProvider(name string) opt.Opt {
	return opt.SetString("provider", name)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// matchProvider returns true if the provider name matches
func matchProvider(o opt.Options, name string) bool {
	if o.Has("provider") && o.GetString("provider") != name {
		return false
	}
	return true
}
