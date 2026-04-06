package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

func TestListAgentsCommandEmbedsRequest(t *testing.T) {
	assert := assert.New(t)
	limit := uint64(10)
	cmd := ListAgentsCommand{AgentListRequest: schema.AgentListRequest{
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

func TestGetAgentCommandName(t *testing.T) {
	assert := assert.New(t)
	cmd := GetAgentCommand{Name: "builtin.alpha"}

	assert.Equal("builtin.alpha", cmd.Name)
}

func TestCallAgentCommandName(t *testing.T) {
	assert := assert.New(t)
	cmd := CallAgentCommand{Name: "builtin.alpha"}

	assert.Equal("builtin.alpha", cmd.Name)
}

func TestCallAgentCommandRequest(t *testing.T) {
	assert := assert.New(t)
	cmd := CallAgentCommand{Name: "builtin.alpha", Input: `{"query":"docs"}`}

	req, err := cmd.requestWithInput(nil, false)
	if assert.NoError(err) {
		assert.Equal(json.RawMessage(`{"query":"docs"}`), req.Input)
	}
}

func TestCallAgentCommandRequestInvalidJSON(t *testing.T) {
	assert := assert.New(t)
	cmd := CallAgentCommand{Name: "builtin.alpha", Input: `{"query":`}

	_, err := cmd.requestWithInput(nil, false)
	assert.Error(err)
}

func TestCallAgentCommandRequestFromStdin(t *testing.T) {
	assert := assert.New(t)
	cmd := CallAgentCommand{Name: "builtin.alpha"}

	req, err := cmd.requestWithInput(bytes.NewBufferString(`{"query":"docs"}`), true)
	if assert.NoError(err) {
		assert.Equal(json.RawMessage(`{"query":"docs"}`), req.Input)
	}
}

func TestCallAgentCommandRequestPrefersArgOverStdin(t *testing.T) {
	assert := assert.New(t)
	cmd := CallAgentCommand{Name: "builtin.alpha", Input: `{"query":"arg"}`}

	req, err := cmd.requestWithInput(bytes.NewBufferString(`{"query":"stdin"}`), true)
	if assert.NoError(err) {
		assert.Equal(json.RawMessage(`{"query":"arg"}`), req.Input)
	}
}

func TestCallAgentCommandRequestInvalidStdinJSON(t *testing.T) {
	assert := assert.New(t)
	cmd := CallAgentCommand{Name: "builtin.alpha"}

	_, err := cmd.requestWithInput(bytes.NewBufferString(`{"query":`), true)
	assert.Error(err)
}
