package main

import (
	"fmt"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	"go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type EmbeddingCommand struct {
	schema.EmbeddingRequest
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *EmbeddingCommand) Run(ctx *Globals) (err error) {
	// Load defaults for model and provider when not explicitly set
	if cmd.EmbeddingRequest.Model == "" {
		cmd.EmbeddingRequest.Model = ctx.defaults.GetString("embedding_model")
	}
	if cmd.EmbeddingRequest.Provider == "" {
		cmd.EmbeddingRequest.Provider = ctx.defaults.GetString("embedding_provider")
	}
	if cmd.EmbeddingRequest.Model == "" {
		return fmt.Errorf("model is required (set with --model or store a default)")
	}

	// Store model and provider as defaults
	if err := ctx.defaults.Set("embedding_model", cmd.EmbeddingRequest.Model); err != nil {
		return err
	}
	if cmd.EmbeddingRequest.Provider != "" {
		if err := ctx.defaults.Set("embedding_provider", cmd.EmbeddingRequest.Provider); err != nil {
			return err
		}
	}

	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "EmbeddingCommand",
		attribute.String("request", types.Stringify(cmd)),
	)
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
