package mistral_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	// Packages

	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

func Test_completion_001(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "mistral-small-latest")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	response, err := model.Completion(context.TODO(), "Hello, how are you?")
	if assert.NoError(err) {
		assert.NotEmpty(response)
		t.Log(response)
	}
}

func Test_completion_004(t *testing.T) {
	assert := assert.New(t)

	model := client.Model(context.TODO(), "mistral-small-latest")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	// Test tool support
	t.Run("Toolkit", func(t *testing.T) {
		toolkit := tool.NewToolKit()
		toolkit.Register(&weather{})
		session := model.Context(llm.WithToolKit(toolkit))

		assert.NoError(session.FromUser(context.TODO(), "What is the weather in the capital city of Germany?"))

		assert.Equal("assistant", session.Role())
		assert.Equal(1, session.Num())
		assert.NotEmpty(session.ToolCalls(0))

		results, err := toolkit.Run(context.TODO(), session.ToolCalls(0)...)
		assert.NoError(err)
		assert.NotEmpty(results)

		assert.NoError(session.FromTool(context.TODO(), results...))

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

func Test_completion_005(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "pixtral-12b-2409")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	// Test image captioning
	t.Run("ImageCaption", func(t *testing.T) {
		f, err := os.Open("../../etc/testdata/guggenheim.jpg")
		if !assert.NoError(err) {
			t.FailNow()
		}
		defer f.Close()

		r, err := model.Completion(
			context.TODO(),
			"Describe this picture",
			llm.WithAttachment(f),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
		}
	})
}

/*
func Test_chat_002(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "mistral-small-latest")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	t.Run("Temperature", func(t *testing.T) {
		r, err := client.Completion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithTemperature(0.5))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("TopP", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithTopP(0.5))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("MaxTokens", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithMaxTokens(10))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("Stream", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithNumCompletions(2), llm.WithStream(func(r llm.Completion) {
			t.Log(r.Role(), "=>", r.Text(0))
		}))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(2, r.Num())
			assert.NotEmpty(r.Text(0))
			assert.NotEmpty(r.Text(1))
			t.Log(r)
		}
	})
	t.Run("Stop", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithStopSequence("STOP"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("System", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithSystemPrompt("You are shakespearian"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("Seed", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithSeed(123))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("Format", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithFormat("json_object"), llm.WithSystemPrompt("Return a JSON object"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("ToolChoiceAuto", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithToolChoice("auto"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("ToolChoiceFunc", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithToolChoice("get_weather"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("PresencePenalty", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithPresencePenalty(-2))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("FrequencyPenalty", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithFrequencyPenalty(-2))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("NumChoices", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithNumCompletions(3))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(3, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("Prediction", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), mistral.WithPrediction("The temperature in London today is"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("SafePrompt", func(t *testing.T) {
		r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithSafePrompt())
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
}

func Test_chat_003(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "pixtral-12b-2409")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	f, err := os.Open("testdata/guggenheim.jpg")
	if !assert.NoError(err) {
		t.FailNow()
	}
	defer f.Close()

	// Describe an image
	r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("Provide a short caption for this image", llm.WithAttachment(f)))
	if assert.NoError(err) {
		assert.Equal("assistant", r.Role())
		assert.Equal(1, r.Num())
		assert.NotEmpty(r.Text(0))
		t.Log(r.Text(0))
	}
}

func Test_chat_004(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "mistral-small-latest")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	toolkit := tool.NewToolKit()
	toolkit.Register(&weather{})

	// Get the weather for a city
	r, err := client.ChatCompletion(context.TODO(), model.UserPrompt("What is the weather in the capital city of germany?"), llm.WithToolKit(toolkit))
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
	return fmt.Sprintf("The weather in %q is sunny and warm", w.City), nil
}
*/
