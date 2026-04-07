package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	resource "github.com/mutablelogic/go-llm/toolkit/resource"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

func TestListToolsCommandEmbedsRequest(t *testing.T) {
	assert := assert.New(t)
	limit := uint64(10)
	cmd := ListToolsCommand{ToolListRequest: schema.ToolListRequest{
		OffsetLimit: pg.OffsetLimit{Limit: &limit, Offset: 5},
		Namespace:   "builtin",
		Name:        []string{"builtin.alpha", "builtin.bravo"},
	}}

	assert.Equal(uint64(5), cmd.Offset)
	if assert.NotNil(cmd.Limit) {
		assert.Equal(uint64(10), *cmd.Limit)
	}
	assert.Equal("builtin", cmd.Namespace)
	assert.Equal([]string{"builtin.alpha", "builtin.bravo"}, cmd.Name)
}

func TestGetToolCommandName(t *testing.T) {
	assert := assert.New(t)
	cmd := GetToolCommand{Name: "builtin.alpha"}

	assert.Equal("builtin.alpha", cmd.Name)
}

func TestCallToolCommandRequest(t *testing.T) {
	assert := assert.New(t)
	cmd := CallToolCommand{Name: "builtin.alpha", Input: `{"query":"docs"}`}

	req, err := cmd.requestWithInput(nil, false)
	if assert.NoError(err) {
		assert.Equal(json.RawMessage(`{"query":"docs"}`), req.Input)
	}
}

func TestCallToolCommandRequestInvalidJSON(t *testing.T) {
	assert := assert.New(t)
	cmd := CallToolCommand{Name: "builtin.alpha", Input: `{"query":`}

	_, err := cmd.requestWithInput(nil, false)
	assert.Error(err)
}

func TestCallToolCommandRequestFromStdin(t *testing.T) {
	assert := assert.New(t)
	cmd := CallToolCommand{Name: "builtin.alpha"}

	req, err := cmd.requestWithInput(bytes.NewBufferString(`{"query":"docs"}`), true)
	if assert.NoError(err) {
		assert.Equal(json.RawMessage(`{"query":"docs"}`), req.Input)
	}
}

func TestCallToolCommandRequestPrefersArgOverStdin(t *testing.T) {
	assert := assert.New(t)
	cmd := CallToolCommand{Name: "builtin.alpha", Input: `{"query":"arg"}`}

	req, err := cmd.requestWithInput(bytes.NewBufferString(`{"query":"stdin"}`), true)
	if assert.NoError(err) {
		assert.Equal(json.RawMessage(`{"query":"arg"}`), req.Input)
	}
}

func TestCallToolCommandRequestInvalidStdinJSON(t *testing.T) {
	assert := assert.New(t)
	cmd := CallToolCommand{Name: "builtin.alpha"}

	_, err := cmd.requestWithInput(bytes.NewBufferString(`{"query":`), true)
	assert.Error(err)
}

func TestWriteToolResourceJSON(t *testing.T) {
	assert := assert.New(t)
	resource := resource.Must("alpha", json.RawMessage(`{"query":"docs"}`))
	var buf bytes.Buffer

	err := writeToolResource(context.Background(), &buf, resource)
	if assert.NoError(err) {
		assert.Equal("{\n  \"query\": \"docs\"\n}\n", buf.String())
	}
}

func TestWriteToolResourceTextAddsNewline(t *testing.T) {
	assert := assert.New(t)
	resource := resource.Must("note", "hello")
	var buf bytes.Buffer

	err := writeToolResource(context.Background(), &buf, resource)
	if assert.NoError(err) {
		assert.Equal("hello\n", buf.String())
	}
}
