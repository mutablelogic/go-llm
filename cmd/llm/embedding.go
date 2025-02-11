package main

import (
	"context"
	"encoding/json"
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
	return run(globals, EmbeddingsType, cmd.Model, func(ctx context.Context, model llm.Model) error {
		vector, err := model.Embedding(ctx, cmd.Prompt)
		if err != nil {
			return err
		}
		data, err := json.Marshal(vector)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	})
}
