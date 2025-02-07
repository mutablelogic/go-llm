package main

import (
	"context"
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type EmbeddingCmd struct {
	Model  string `arg:"" help:"Model name"`
	Prompt string `arg:"" help:"Prompt"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *EmbeddingCmd) Run(globals *Globals) error {
	return run(globals, cmd.Model, func(ctx context.Context, model llm.Model) error {
		fmt.Println(model)
		return nil
	})
}
