package main

import (
	"context"
	"fmt"

	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type ListModelsCmd struct {
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*ListModelsCmd) Run(globals *Globals) error {
	return runagent(globals, func(ctx context.Context, agent llm.Agent) error {
		models, err := agent.Models(ctx)
		if err != nil {
			return err
		}
		fmt.Println(models)
		return nil
	})
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func runagent(globals *Globals, fn func(ctx context.Context, agent llm.Agent) error) error {
	return fn(globals.ctx, globals.agent)
}
