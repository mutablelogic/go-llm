package main

import (
	"context"
	"encoding/json"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	tool "github.com/mutablelogic/go-llm/pkg/toolkit/tool"
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

func (*greetTool) InputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[greetRequest](nil)
}

func (*greetTool) OutputSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[greetResponse](nil)
}

func (*greetTool) Run(_ context.Context, input json.RawMessage) (any, error) {
	var req greetRequest
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.Withf("greet: %v", err)
		}
	}
	if req.Name == "" {
		return nil, llm.ErrBadParameter.Withf("greet: name is required")
	}
	resp := greetResponse{
		Name:     req.Name,
		Greeting: "Hello, " + req.Name + "!",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
