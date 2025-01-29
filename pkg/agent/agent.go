package agent

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type agent struct {
	*opt
}

var _ llm.Agent = (*agent)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return a new agent, composed of several different models from different providers
func New(opts ...llm.Opt) (*agent, error) {
	agent := new(agent)
	opt, err := apply(opts...)
	if err != nil {
		return nil, err
	} else {
		agent.opt = opt
	}

	// Return success
	return agent, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS
