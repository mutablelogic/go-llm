package ollama_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	// Packages

	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

func Test_completion_001(t *testing.T) {
	assert := assert.New(t)

	// Pull the model
	model, err := client.PullModel(context.TODO(), "qwen:0.5b", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	}))
	if err != nil {
		t.FailNow()
	}

	// Get a completion
	response, err := model.Completion(context.TODO(), "Hello, how are you?")
	if assert.NoError(err) {
		assert.NotEmpty(response)
	}
}

func Test_completion_002(t *testing.T) {
	assert := assert.New(t)

	// Pull the model
	model, err := client.PullModel(context.TODO(), "qwen:0.5b", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	}))
	if err != nil {
		t.FailNow()
	}

	t.Run("Temperature", func(t *testing.T) {
		response, err := model.Completion(context.TODO(), "Tell me in less than five words why the sky is blue?", llm.WithTemperature(0.5))
		if assert.NoError(err) {
			t.Log(response)
		}
	})

	t.Run("TopP", func(t *testing.T) {
		response, err := model.Completion(context.TODO(), "Tell me in less than five words why the sky is blue?", llm.WithTopP(0.5))
		if assert.NoError(err) {
			t.Log(response)
		}
	})

	t.Run("TopK", func(t *testing.T) {
		response, err := model.Completion(context.TODO(), "Tell me in less than five words why the sky is blue?", llm.WithTopK(50))
		if assert.NoError(err) {
			t.Log(response)
		}
	})

	t.Run("Stop", func(t *testing.T) {
		response, err := model.Completion(context.TODO(), "Tell me in less than five words why the sky is blue?", llm.WithStopSequence("sky"))
		if assert.NoError(err) {
			t.Log(response)
		}
	})

	t.Run("System", func(t *testing.T) {
		response, err := model.Completion(context.TODO(), "Tell me in less than five words why the sky is blue?", llm.WithSystemPrompt("reply as if you are shakespeare"))
		if assert.NoError(err) {
			t.Log(response)
		}
	})

	t.Run("Seed", func(t *testing.T) {
		response, err := model.Completion(context.TODO(), "Tell me in less than five words why the sky is blue?", llm.WithSeed(123))
		if assert.NoError(err) {
			t.Log(response)
		}
	})

	t.Run("Format", func(t *testing.T) {
		response, err := model.Completion(context.TODO(), "Why the sky is blue? Reply in JSON format", llm.WithFormat("json"))
		if assert.NoError(err) {
			t.Log(response)
		}
	})

	t.Run("FrequencyPenalty", func(t *testing.T) {
		response, err := model.Completion(context.TODO(), "Why the sky is blue?", llm.WithFrequencyPenalty(1.0))
		if assert.NoError(err) {
			t.Log(response)
		}
	})
}

func Test_completion_003(t *testing.T) {
	assert := assert.New(t)

	// Pull the model
	model, err := client.PullModel(context.TODO(), "llama3.2-vision", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	}))
	if err != nil {
		t.FailNow()
	}

	t.Run("Vision", func(t *testing.T) {
		f, err := os.Open("testdata/guggenheim.jpg")
		if !assert.NoError(err) {
			t.FailNow()
		}
		defer f.Close()
		response, err := model.Completion(context.TODO(), "Describe this image", llm.WithAttachment(f))
		if assert.NoError(err) {
			t.Log(response)
		}
	})
}

func Test_completion_004(t *testing.T) {
	assert := assert.New(t)

	// Pull the model
	model, err := client.PullModel(context.TODO(), "mistral", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	}))
	if err != nil {
		t.FailNow()
	}

	// Test tool support
	t.Run("Toolkit", func(t *testing.T) {
		toolkit := tool.NewToolKit()
		toolkit.Register(&weather{})

		session := model.Context(llm.WithToolKit(toolkit))
		err := session.FromUser(context.TODO(),
			"What is the weather in the capital city of Germany?",
		)
		if !assert.NoError(err) {
			t.FailNow()
		}

		assert.Equal("assistant", session.Role())
		assert.Greater(session.Num(), 0)
		assert.NotEmpty(session.ToolCalls(0))

		toolcalls := session.ToolCalls(0)
		assert.NotEmpty(toolcalls)
		assert.Equal("weather_in_city", toolcalls[0].Name())

		results, err := toolkit.Run(context.TODO(), toolcalls...)
		if !assert.NoError(err) {
			t.FailNow()
		}

		assert.Len(results, len(toolcalls))

		err = session.FromTool(context.TODO(), results...)
		if !assert.NoError(err) {
			t.FailNow()
		}
	})
}

type weather struct {
	City string `json:"city" help:"The city to get the weather for" required:"true"`
}

func (weather) Name() string {
	return "weather_in_city"
}

func (weather) Description() string {
	return "Get the weather for a city"
}

func (w weather) Run(ctx context.Context) (any, error) {
	return fmt.Sprintf("The weather in %q is sunny and warm", w.City), nil
}
