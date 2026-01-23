package main

import (
	"encoding/json"
	"fmt"
	"os"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type EmbeddingCommands struct {
	Embedding EmbeddingCommand `cmd:"" name:"embedding" help:"Generate embedding for a file." group:"EMBEDDING"`
}

type EmbeddingCommand struct {
	Model string `arg:"" name:"model" help:"Model name to use for embedding"`
	File  string `arg:"" name:"file" help:"File to generate embedding for"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *EmbeddingCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Agent()
	if err != nil {
		return err
	}

	// Check if the client implements the Embedder interface
	embedder, ok := client.(llm.Embedder)
	if !ok {
		return fmt.Errorf("client does not support embeddings")
	}

	// OTEL tracing
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "EmbeddingCommand")
	defer func() { endSpan(err) }()

	// Get the model
	model, err := client.GetModel(parent, cmd.Model)
	if err != nil {
		return fmt.Errorf("failed to get model %q: %w", cmd.Model, err)
	}

	// Read the file
	data, err := os.ReadFile(cmd.File)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", cmd.File, err)
	}

	// Generate embedding
	vector, err := embedder.Embedding(parent, *model, string(data))
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Output the embedding as JSON
	output, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	fmt.Println(string(output))
	return nil
}
