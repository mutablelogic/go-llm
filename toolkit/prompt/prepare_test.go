package prompt_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	prompt "github.com/mutablelogic/go-llm/toolkit/prompt"
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

func TestPrepare_001(t *testing.T) {
	assert := assert.New(t)
	p := mustReadPrompt(t, "greeter.md", "---\nname: greeter\n---\nHello, {{ .name }}!")

	text, opts, err := p.Prepare(context.Background(), json.RawMessage(`{"name":"World"}`))
	assert.NoError(err)
	assert.Equal("Hello, World!", text)
	assert.Nil(opts)
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

	_, _, err := p.Prepare(context.Background(), json.RawMessage(`{"text":"Hello"}`))
	assert.Error(err)
	assert.Contains(err.Error(), "input validation")
}

func TestPrepare_003_TemplateFunctions(t *testing.T) {
	assert := assert.New(t)
	p := mustReadPrompt(t, "funcs.md", `---
name: funcs
---
{{ upper (trim .name) }}|{{ default "fallback" .missing }}|{{ join .items "," }}|{{ json .items }}`)

	text, _, err := p.Prepare(context.Background(), json.RawMessage(`{"name":"  world  ","items":["a","b"]}`))
	assert.NoError(err)
	assert.Equal("WORLD|fallback|a,b|[\"a\",\"b\"]", text)
}

func TestPrepare_004_InvalidJSON(t *testing.T) {
	assert := assert.New(t)
	p := mustReadPrompt(t, "greeter.md", "---\nname: greeter\n---\nHello, {{ .name }}!")

	_, _, err := p.Prepare(context.Background(), json.RawMessage(`{"name":`))
	assert.Error(err)
	assert.Contains(err.Error(), "invalid JSON")
}

func TestPrepare_005_InvalidTemplate(t *testing.T) {
	assert := assert.New(t)
	p := mustReadPrompt(t, "bad.md", "---\nname: bad\n---\n{{ .name")

	_, _, err := p.Prepare(context.Background(), json.RawMessage(`{"name":"World"}`))
	assert.Error(err)
	assert.Contains(err.Error(), "template")
}
