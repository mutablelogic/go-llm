package agent

import (
	// Packages
	"github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/ollama"
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
// OPTIONS

// Ollama
func WithOllama(endpoint string, opts ...client.ClientOpt) llm.Opt {
	return func(o any) error {
		client, err := ollama.New(endpoint, opts...)
		if err != nil {
			return err
		}
		name := client.Name()
		if _, exists := o.(*opt).agents[name]; exists {
			return llm.ErrConflict.Withf("Agent %q already exists", name)
		} else {
			o.(*opt).agents[name] = client
		}
		return nil
	}
}
