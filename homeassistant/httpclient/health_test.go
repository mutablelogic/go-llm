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

func Test_health_001(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)

	message, err := client.Health(context.Background())
	assert.NoError(err)
	assert.NotEmpty(message)

	t.Log(message)
}
