package prompt_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	prompt "github.com/mutablelogic/go-llm/toolkit/prompt"
	resource "github.com/mutablelogic/go-llm/toolkit/resource"
	assert "github.com/stretchr/testify/assert"
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

func TestPrepare_002_ValidatesInput(t *testing.T) {
	assert := assert.New(t)
	p := mustReadPrompt(t, "translate.md", `---
name: translate
input:
  type: object
  properties:
    text:
      type: string
    target_language:
      type: string
  required:
    - text
    - target_language
---
Translate: {{ .text }}`)

	_, _, err := p.Prepare(context.Background(), resource.Must("input", json.RawMessage(`{"text":"Hello"}`)))
	assert.Error(err)
	assert.Contains(err.Error(), "input validation")
}

func TestPrepare_005_InvalidTemplate(t *testing.T) {
	assert := assert.New(t)
	p := mustReadPrompt(t, "bad.md", "---\nname: bad\n---\n{{ .name")

	_, _, err := p.Prepare(context.Background(), resource.Must("input", json.RawMessage(`{"name":"World"}`)))
	assert.Error(err)
	assert.Contains(err.Error(), "template")
}
