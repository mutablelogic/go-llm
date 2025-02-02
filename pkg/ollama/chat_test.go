package ollama_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	// Packages

	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
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

	t.Run("System", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), llm.WithSystemPrompt("reply as if you are shakespeare"))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("Seed", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), llm.WithSeed(1234))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("Format", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue? Reply in JSON format"), llm.WithFormat("json"))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("PresencePenalty", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?t"), llm.WithPresencePenalty(-1.0))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("FrequencyPenalty", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?t"), llm.WithFrequencyPenalty(1.0))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})
}

func Test_chat_002(t *testing.T) {
	assert := assert.New(t)
	model, err := client.PullModel(context.TODO(), "llava:7b")
	if !assert.NoError(err) {
		t.FailNow()
	}
	assert.NotNil(model)

	f, err := os.Open("testdata/guggenheim.jpg")
	if !assert.NoError(err) {
		t.FailNow()
	}
	defer f.Close()

	// Describe an image
	r, err := client.Chat(context.TODO(), model.UserPrompt("Provide a short caption for this image", llm.WithAttachment(f)))
	if assert.NoError(err) {
		assert.Equal("assistant", r.Role())
		assert.Equal(1, r.Num())
		assert.NotEmpty(r.Text(0))
		t.Log(r.Text(0))
	}
}

func Test_chat_003(t *testing.T) {
	assert := assert.New(t)
	model, err := client.PullModel(context.TODO(), "llama3.2")
	if !assert.NoError(err) {
		t.FailNow()
	}
	assert.NotNil(model)

	toolkit := tool.NewToolKit()
	toolkit.Register(&weather{})

	// Get the weather for a city
	r, err := client.Chat(context.TODO(), model.UserPrompt("What is the weather in the capital city of germany?"), llm.WithToolKit(toolkit))
	if assert.NoError(err) {
		assert.Equal("assistant", r.Role())
		assert.Equal(1, r.Num())

		calls := r.ToolCalls(0)
		assert.NotEmpty(calls)

		var w weather
		assert.NoError(calls[0].Decode(&w))
		assert.Equal("berlin", strings.ToLower(w.City))
	}
}

type weather struct {
	City string `json:"city" help:"The city to get the weather for"`
}

func (weather) Name() string {
	return "weather_in_city"
}

func (weather) Description() string {
	return "Get the weather for a city"
}

func (w weather) Run(ctx context.Context) (any, error) {
	var result struct {
		City    string `json:"city"`
		Weather string `json:"weather"`
	}
	result.City = w.City
	result.Weather = fmt.Sprintf("The weather in %q is sunny and warm", w.City)
	return result, nil
}
