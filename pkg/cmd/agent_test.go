package cmd

import (
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
