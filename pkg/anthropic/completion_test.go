package anthropic_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	// Packages

	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	"github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

func Test_chat_001(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "claude-3-5-haiku-20241022")

	if assert.NotNil(model) {
		response, err := client.Messages(context.TODO(), model.UserPrompt("Hello, how are you?"))
		assert.NoError(err)
		assert.NotEmpty(response)
		t.Log(response)
	}
}

func Test_chat_002(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "claude-3-5-haiku-20241022")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	t.Run("Temperature", func(t *testing.T) {
		r, err := client.Messages(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithTemperature(0.5))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("TopP", func(t *testing.T) {
		r, err := client.Messages(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithTopP(0.5))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("TopK", func(t *testing.T) {
		r, err := client.Messages(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithTopK(90))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})

	t.Run("MaxTokens", func(t *testing.T) {
		r, err := client.Messages(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithMaxTokens(10))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("Stream", func(t *testing.T) {
		r, err := client.Messages(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithStream(func(r llm.Completion) {
			t.Log(r.Role(), "=>", r.Text(0))
		}))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("Stop", func(t *testing.T) {
		r, err := client.Messages(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithStopSequence("weather"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("System", func(t *testing.T) {
		r, err := client.Messages(context.TODO(), model.UserPrompt("What is the temperature in London?"), llm.WithSystemPrompt("You reply in shakespearian language, in one sentence"))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r)
		}
	})
	t.Run("User", func(t *testing.T) {
		r, err := client.Messages(context.TODO(), model.UserPrompt("What is the temperature in London?"), anthropic.WithUser("username"))
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
	model := client.Model(context.TODO(), "claude-3-5-sonnet-20241022")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	t.Run("ImageCaption", func(t *testing.T) {
		f, err := os.Open("testdata/guggenheim.jpg")
		if !assert.NoError(err) {
			t.FailNow()
		}
		defer f.Close()

		// Describe an image
		r, err := client.Messages(context.TODO(), model.UserPrompt("Provide a short caption for this image", llm.WithAttachment(f)))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r.Text(0))
		}
	})

	t.Run("DocSummarize", func(t *testing.T) {
		f, err := os.Open("testdata/LICENSE")
		if !assert.NoError(err) {
			t.FailNow()
		}
		defer f.Close()

		// Summarize a document
		r, err := client.Messages(context.TODO(), model.UserPrompt("Summarize this document", llm.WithAttachment(f)))
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())
			assert.NotEmpty(r.Text(0))
			t.Log(r.Text(0))
		}
	})
}

func Test_chat_004(t *testing.T) {
	assert := assert.New(t)
	model := client.Model(context.TODO(), "claude-3-5-haiku-20241022")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	toolkit := tool.NewToolKit()
	toolkit.Register(&weather{})

	t.Run("ToolChoiceAuto", func(t *testing.T) {
		// Get the weather for a city
		r, err := client.Messages(
			context.TODO(),
			model.UserPrompt("What is the weather in the capital city of germany?"),
			llm.WithToolKit(toolkit),
			llm.WithToolChoice("auto"),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())

			calls := r.ToolCalls(0)
			assert.NotEmpty(calls)

			var w weather
			assert.NoError(calls[0].Decode(&w))
			assert.Equal("berlin", strings.ToLower(w.City))
		}
	})
	t.Run("ToolChoiceFunc", func(t *testing.T) {
		// Get the weather for a city
		r, err := client.Messages(
			context.TODO(),
			model.UserPrompt("What is the weather in the capital city of germany?"),
			llm.WithToolKit(toolkit),
			llm.WithToolChoice("weather_in_city"),
		)
		if assert.NoError(err) {
			assert.Equal("assistant", r.Role())
			assert.Equal(1, r.Num())

			calls := r.ToolCalls(0)
			assert.NotEmpty(calls)

			var w weather
			assert.NoError(calls[0].Decode(&w))
			assert.Equal("berlin", strings.ToLower(w.City))
		}
	})
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
