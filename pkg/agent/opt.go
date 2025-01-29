package agent

import (
	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type opt struct {
	agents map[string]llm.Agent
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func apply(opts ...llm.Opt) (*opt, error) {
	o := new(opt)
	o.agents = make(map[string]llm.Agent)
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func WithOllama(endpoint string, opts ...client.ClientOpt) llm.Opt {
	return func(o any) error {
		client, err := ollama.New(endpoint, opts...)
		if err != nil {
			return err
		} else {
			return o.(*opt).withAgent(client)
		}
	}
}

func WithAnthropic(key string, opts ...client.ClientOpt) llm.Opt {
	return func(o any) error {
		client, err := anthropic.New(key, opts...)
		if err != nil {
			return err
		} else {
			return o.(*opt).withAgent(client)
		}
	}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (o *opt) withAgent(agent llm.Agent) error {
	name := agent.Name()
	if _, exists := o.agents[name]; exists {
		return llm.ErrConflict.Withf("Agent %q already exists", name)
	} else {
		o.agents[name] = agent
	}
	// Return success
	return nil
}
