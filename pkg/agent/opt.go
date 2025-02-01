package agent

import (
	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func WithOllama(endpoint string, opts ...client.ClientOpt) llm.Opt {
	return func(o *llm.Opts) error {
		client, err := ollama.New(endpoint, opts...)
		if err != nil {
			return err
		} else {
			return llm.WithAgent(client)(o)
		}
	}
}

func WithAnthropic(key string, opts ...client.ClientOpt) llm.Opt {
	return func(o *llm.Opts) error {
		client, err := anthropic.New(key, opts...)
		if err != nil {
			return err
		} else {
			return llm.WithAgent(client)(o)
		}
	}
}
