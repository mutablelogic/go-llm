package server_test

import (
	"bytes"
	"context"
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	server "github.com/mutablelogic/go-llm/mcp/server"
	prompt "github.com/mutablelogic/go-llm/toolkit/prompt"
)

type namedReader struct {
	*bytes.Reader
	name string
}

func (r *namedReader) Name() string {
	return r.name
}

func mustReadPrompt(t *testing.T, name, body string) llm.Prompt {
	t.Helper()
	p, err := prompt.Read(&namedReader{Reader: bytes.NewReader([]byte(body)), name: name})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestServerListPrompts(t *testing.T) {
	srv, err := server.New("test-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	prompt := mustReadPrompt(t, "greet.md", `---
name: greet
title: Greeting Prompt
description: Greets someone by name
input:
  type: object
  properties:
    name:
      description: Name to greet
      type: string
  required:
    - name
---
Hello, {{ .name }}!`)
	srv.AddPrompts(prompt)

	_, session := connect(t, srv)

	result, err := session.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(result.Prompts))
	}
	p := result.Prompts[0]
	if p.Name != "greet" {
		t.Errorf("expected prompt name %q, got %q", "greet", p.Name)
	}
	if p.Title != "Greeting Prompt" {
		t.Errorf("expected prompt title %q, got %q", "Greeting Prompt", p.Title)
	}
	if len(p.Arguments) != 1 || p.Arguments[0].Name != "name" {
		t.Errorf("expected 1 argument named %q, got %v", "name", p.Arguments)
	}
	if !p.Arguments[0].Required {
		t.Errorf("expected argument %q to be required", "name")
	}
}

func TestServerGetPrompt(t *testing.T) {
	srv, err := server.New("test-server", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	prompt := mustReadPrompt(t, "greet.md", `---
name: greet
title: Greeting Prompt
description: Greets someone by name
input:
  type: object
  properties:
    name:
      description: Name to greet
      type: string
  required:
    - name
---
Hello, {{ .name }}!`)
	srv.AddPrompts(prompt)

	_, session := connect(t, srv)

	result, err := session.GetPrompt(context.Background(), &sdkmcp.GetPromptParams{
		Name:      "greet",
		Arguments: map[string]string{"name": "World"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	msg := result.Messages[0]
	if msg.Role != "user" {
		t.Errorf("expected role %q, got %q", "user", msg.Role)
	}
	text, ok := msg.Content.(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.TextContent, got %T", msg.Content)
	}
	if text.Text != "Hello, World!" {
		t.Errorf("expected rendered text %q, got %q", "Hello, World!", text.Text)
	}
}
