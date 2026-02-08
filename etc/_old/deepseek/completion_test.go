package deepseek_test

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

func Test_completion_001(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "deepseek-chat")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	response, err := model.Completion(context.TODO(), "Hello, how are you?")
	if assert.NoError(err) {
		assert.NotEmpty(response)
		t.Log(response)
	}
}

func Test_completion_002(t *testing.T) {
	assert := assert.New(t)

	// Test options
	model := client.Model(context.TODO(), "deepseek-chat")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	t.Run("FrequencyPenalty", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", llm.WithFrequencyPenalty(-0.5))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("LogProbs", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", llm.WithLogProbs())
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("TopLogProbs", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", llm.WithTopLogProbs(3))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("MaxTokens", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", llm.WithMaxTokens(20))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("Completions", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", llm.WithNumCompletions(3))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(3, r.Num())
			assert.NotEmpty(r.Text(0))
			assert.NotEmpty(r.Text(1))
			assert.NotEmpty(r.Text(2))
			t.Log(r)
		}
	})

	t.Run("PresencePenalty", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			llm.WithPresencePenalty(1.0),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("ResponseFormat", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue, and response in JSON format",
			llm.WithFormat("json_object"),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("Stop", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			llm.WithStopSequence("sky", "blue"),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("TopP", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			llm.WithTopP(0.1),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
}
