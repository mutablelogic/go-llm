package mistral_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
	mistral "github.com/mutablelogic/go-llm/pkg/mistral"
	assert "github.com/stretchr/testify/assert"
)

func Test_chat_001(t *testing.T) {
	assert := assert.New(t)

	client, err := mistral.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	model := client.Model(context.TODO(), "mistral-small-latest")
	if assert.NotNil(model) {
		response, err := client.ChatCompletion(context.TODO(), model.UserPrompt("Hello, how are you?"))
		assert.NoError(err)
		assert.NotEmpty(response)
		t.Log(response)
	}
}

func Test_chat_002(t *testing.T) {
	assert := assert.New(t)

	client, err := mistral.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	model := client.Model(context.TODO(), "mistral-large-latest")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	t.Run("Temperature", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithTemperature(0.5))
		assert.NoError(err)
	})
	t.Run("TopP", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithTopP(0.5))
		assert.NoError(err)
	})
	t.Run("MaxTokens", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithMaxTokens(10))
		assert.NoError(err)
	})
	t.Run("Stream", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithStream(func(r llm.ContextContent) {
			t.Log(r.Role(), "=>", r.Text())
		}))
		assert.NoError(err)
	})
	t.Run("Stop", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithStopSequence("STOP"))
		assert.NoError(err)
	})
	t.Run("System", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithSystemPrompt("You are shakespearian"))
		assert.NoError(err)
	})
	t.Run("Seed", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithSeed(123))
		assert.NoError(err)
	})
	t.Run("Format", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithFormat("json_object"), llm.WithSystemPrompt("Return a JSON object"))
		assert.NoError(err)
	})
	t.Run("ToolChoiceAuto", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithToolChoice("auto"))
		assert.NoError(err)
	})
	t.Run("ToolChoiceFunc", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithToolChoice("get_weather"))
		assert.NoError(err)
	})
	t.Run("PresencePenalty", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), mistral.WithPresencePenalty(-2))
		assert.NoError(err)
	})
	t.Run("FrequencyPenalty", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), mistral.WithFrequencyPenalty(-2))
		assert.NoError(err)
	})
	t.Run("NumChoices", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithNumCompletions(3))
		assert.NoError(err)
	})
	t.Run("Prediction", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), mistral.WithPrediction("The temperature in London today is"))
		assert.NoError(err)
	})
	t.Run("SafePrompt", func(t *testing.T) {
		_, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithSafePrompt())
		assert.NoError(err)
	})
}
