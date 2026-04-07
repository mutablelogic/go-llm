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

func Test_events_001(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)

	events, err := client.Events(context.Background())
	assert.NoError(err)
	assert.NotNil(events)

	t.Log(events)
}

// Test firing a custom event
func Test_events_002(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	msg, err := client.FireEvent(context.Background(), "go_llm_test_event", map[string]any{
		"source": "go-llm",
	})
	assert.NoError(err)
	assert.NotEmpty(msg)
	t.Log("FireEvent:", msg)
}
