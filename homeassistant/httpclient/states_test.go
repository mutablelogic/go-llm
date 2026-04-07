package homeassistant_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	assert "github.com/stretchr/testify/assert"
)

func Test_states_001(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)

	states, err := client.States(context.Background())
	assert.NoError(err)
	assert.NotNil(states)

	for _, state := range states {
		t.Log("State:", state)
		t.Logf("  Value: %q", state.Value())
		t.Log("  Name:", state.Name())
		t.Log("  Domain:", state.Domain())
		t.Log("  Class:", state.Class())
		if unit := state.UnitOfMeasurement(); unit != "" {
			t.Logf("  UnitOfMeasurement: %q", unit)
		}
	}
}

// Test SetState creates/updates an entity and DeleteState removes it
func Test_states_002(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	// Create a test entity
	state, err := client.SetState(context.Background(), "sensor.go_llm_test", "42", map[string]any{
		"unit_of_measurement": "°C",
		"friendly_name":       "Go LLM Test Sensor",
	})
	assert.NoError(err)
	assert.NotNil(state)
	assert.Equal("42", state.State)
	t.Log("Created:", state)

	// Read it back
	got, err := client.State(context.Background(), "sensor.go_llm_test")
	assert.NoError(err)
	assert.Equal("42", got.State)

	// Delete it
	err = client.DeleteState(context.Background(), "sensor.go_llm_test")
	assert.NoError(err)
	t.Log("Deleted sensor.go_llm_test")
}
