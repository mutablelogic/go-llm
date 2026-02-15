package main

import (
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type EmbeddingCommands struct {
	Embedding EmbeddingCommand `cmd:"" name:"embedding" help:"Generate embedding vectors from text." group:"EMBEDDING"`
}

type EmbeddingCommand struct {
	schema.EmbeddingRequest
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *EmbeddingCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "EmbeddingCommand")
	defer func() { endSpan(err) }()

	// Get embeddings
	response, err := client.Embedding(parent, cmd.EmbeddingRequest)
	if err != nil {
		return err
	}

	// Print
	fmt.Println(response)
	return nil
}
