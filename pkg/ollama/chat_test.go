package ollama_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	assert "github.com/stretchr/testify/assert"
)

func Test_chat_001(t *testing.T) {
	client, err := ollama.New(GetEndpoint(t), opts.OptTrace(os.Stderr, true))
	if err != nil {
		t.FailNow()
	}

	// Pull the model
	model, err := client.PullModel(context.TODO(), "qwen:0.5b", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	}))
	if err != nil {
		t.FailNow()
	}

	t.Run("ChatStream", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), ollama.WithStream(func(stream *ollama.Response) {
			t.Log(stream)
		}))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("ChatNoStream", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky green?"))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})
}

func Test_chat_002(t *testing.T) {
	client, err := ollama.New(GetEndpoint(t), opts.OptTrace(os.Stderr, true))
	if err != nil {
		t.FailNow()
	}

	// Pull the model
	model, err := client.PullModel(context.TODO(), "llama3.2:1b", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	}))
	if err != nil {
		t.FailNow()
	}

	t.Run("Tools", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(),
			model.UserPrompt("what is the weather in berlin?"),
			ollama.WithTool(ollama.MustTool("get_weather", "Return weather conditions in a location", struct {
				Location string `help:"Location to get weather for" required:""`
			}{})),
		)
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})
}

func Test_chat_003(t *testing.T) {
	client, err := ollama.New(GetEndpoint(t), opts.OptTrace(os.Stderr, false))
	if err != nil {
		t.FailNow()
	}

	// Pull the model
	model, err := client.PullModel(context.TODO(), "llava", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	}))
	if err != nil {
		t.FailNow()
	}

	// Explain the content of an image
	t.Run("Image", func(t *testing.T) {
		assert := assert.New(t)

		f, err := os.Open("testdata/guggenheim.jpg")
		if !assert.NoError(err) {
			t.FailNow()
		}
		defer f.Close()

		response, err := client.Chat(context.TODO(),
			model.UserPrompt("describe this photo to me", ollama.WithData(f)),
		)
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})
}
