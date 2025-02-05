package openai_test

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func Test_completion_001(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "gpt-4o-mini")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	response, err := model.Completion(context.TODO(), "Hello, how are you?")
	if assert.NoError(err) {
		assert.NotEmpty(response)
		t.Log(response)
	}
}
