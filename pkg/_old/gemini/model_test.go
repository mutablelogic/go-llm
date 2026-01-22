package gemini_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func Test_models_001(t *testing.T) {
	assert := assert.New(t)

	response, err := client.ListModels(context.TODO())
	assert.NoError(err)
	assert.NotEmpty(response)

	data, err := json.MarshalIndent(response, "", "  ")
	assert.NoError(err)
	t.Log(string(data))
}
