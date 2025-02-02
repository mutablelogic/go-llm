package ollama_test

import (
	"context"
	"testing"

	// Packages

	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	assert "github.com/stretchr/testify/assert"
)

func Test_chat_001(t *testing.T) {
	// Pull the model
	model, err := client.PullModel(context.TODO(), "qwen:0.5b", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	}))
	if err != nil {
		t.FailNow()
	}

	t.Run("Temperature", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), llm.WithTemperature(0.5))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("TopP", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), llm.WithTopP(0.5))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})
	t.Run("TopK", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), llm.WithTopK(50))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("Stream", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), llm.WithStream(func(stream llm.Completion) {
			t.Log(stream)
		}))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("Stop", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), llm.WithStopSequence("sky"))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})
}
