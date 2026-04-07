package main

import (
	"context"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	tool "github.com/mutablelogic/go-llm/toolkit/tool"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type greetTool struct {
	tool.Base
}

type greetRequest struct {
	Name string `json:"name" jsonschema:"The name of the person to greet."`
}

type greetResponse struct {
	Name     string `json:"name"`
	Greeting string `json:"greeting"`
}

type fetchRequest struct {
	URL        string `json:"url"`
	MaxLength  int    `json:"max_length,omitempty"`
	StartIndex int    `json:"start_index,omitempty"`
	Raw        bool   `json:"raw,omitempty"`
}

var _ llm.Tool = (*greetTool)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateTools returns the builtin tools for the example toolkit.
func CreateTools() ([]llm.Tool, error) {
	return []llm.Tool{
		&greetTool{},
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// llm.Tool INTERFACE

func (*greetTool) Name() string { return "greet" }

func (*greetTool) Description() string {
	return "Greet a person by name."
}

func (*greetTool) InputSchema() *jsonschema.Schema { return jsonschema.MustFor[greetRequest]() }

func (*greetTool) OutputSchema() *jsonschema.Schema { return jsonschema.MustFor[greetResponse]() }

func (*greetTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req greetRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, schema.ErrBadParameter.Withf("greet: %v", err)
	}
	if req.Name == "" {
		return nil, schema.ErrBadParameter.Withf("greet: name is required")
	}

	session := toolkit.SessionFromContext(ctx)
	session.Logger().Info("greet called", "name", req.Name, "session", session)

	resp := greetResponse{
		Name:     req.Name,
		Greeting: "Hello, " + req.Name + "!",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	session.Progress(100, 100, "Completed")

	return json.RawMessage(data), nil
}
