package openai_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	llm "github.com/mutablelogic/go-llm"
	openai "github.com/mutablelogic/go-llm/pkg/openai"
	"github.com/mutablelogic/go-llm/pkg/tool"
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

func Test_completion_002(t *testing.T) {
	assert := assert.New(t)

	// Test options
	model := client.Model(context.TODO(), "gpt-4o-mini")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	o3_model := client.Model(context.TODO(), "o3-mini")
	if !assert.NotNil(o3_model) {
		t.FailNow()
	}

	audio_model := client.Model(context.TODO(), "gpt-4o-audio-preview")
	if !assert.NotNil(audio_model) {
		t.FailNow()
	}

	t.Run("Store", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", openai.WithStore(true))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("ReasoningEffort", func(t *testing.T) {
		r, err := o3_model.Completion(context.TODO(), "What is the temperature in London?", openai.WithReasoningEffort("low"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("Metadata", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", openai.WithMetadata("a", "b"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("FrequencyPenalty", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", llm.WithFrequencyPenalty(-0.5))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("LogitBias", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", openai.WithLogitBias(56, 22))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("LogProbs", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", openai.WithLogProbs())
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("TopLogProbs", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", openai.WithTopLogProbs(3))
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

	t.Run("Modalties", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "What is the temperature in London?", openai.WithModalities("text"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("Prediction", func(t *testing.T) {
		r, err := model.Completion(context.TODO(), "Why is the sky blue", llm.WithPrediction("The sky is blue due to Rayleigh scattering"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("Audio", func(t *testing.T) {
		r, err := audio_model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			openai.WithAudio("ash", "mp3"),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0)) // Returns the audio transcript
			assert.NotEmpty(r.Attachment(0))
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

	t.Run("Seed", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			llm.WithSeed(123),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("ServiceTier", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			openai.WithServiceTier("default"),
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

	t.Run("User", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			llm.WithUser("test_user"),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

}

func Test_completion_003(t *testing.T) {
	assert := assert.New(t)

	model := client.Model(context.TODO(), "gpt-4o-mini")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	audio_model := client.Model(context.TODO(), "gpt-4o-audio-preview")
	if !assert.NotNil(audio_model) {
		t.FailNow()
	}

	// Test streaming
	t.Run("Streaming", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			llm.WithStream(func(message llm.Completion) {
				// TODO
			}),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
		}
	})

	t.Run("StreamingCompletions", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			llm.WithNumCompletions(2),
			llm.WithStream(func(message llm.Completion) {
				// TODO
			}),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(2, r.Num())
			assert.NotEmpty(r.Text(0))
			assert.NotEmpty(r.Text(1))
		}
	})

	t.Run("StreamingUsage", func(t *testing.T) {
		r, err := model.Completion(
			context.TODO(),
			"Tell me in no more than ten words why is the sky blue",
			openai.WithStreamOptions(func(message llm.Completion) {
				// TODO
			}, true),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
		}
	})

	t.Run("StreamingAudio", func(t *testing.T) {
		r, err := audio_model.Completion(
			context.TODO(),
			"Tell me in exactly three words why is the sky blue",
			openai.WithStreamOptions(func(message llm.Completion) {
				// TODO
			}, true),
			openai.WithAudio("ash", "pcm16"),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			assert.NotEmpty(r.Attachment(0))
		}
	})

}

func Test_completion_004(t *testing.T) {
	assert := assert.New(t)

	model := client.Model(context.TODO(), "gpt-4o-mini")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	// Test tool support
	t.Run("Toolkit", func(t *testing.T) {
		toolkit := tool.NewToolKit()
		toolkit.Register(weather{})

		r, err := model.Completion(
			context.TODO(),
			"What is the weather in the capital city of Germany?",
			llm.WithToolKit(toolkit),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.ToolCalls(0))

			toolcalls := r.ToolCalls(0)
			assert.Len(toolcalls, 1)
			assert.Equal("weather_in_city", toolcalls[0].Name())
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

func Test_completion_005(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "gpt-4o-mini")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	// Test image captioning
	t.Run("ImageCaption", func(t *testing.T) {
		f, err := os.Open("testdata/guggenheim.jpg")
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
