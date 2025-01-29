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
	if err := client.PullModel(context.TODO(), "qwen:0.5b", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	})); err != nil {
		t.FailNow()
	}

	t.Run("ChatStream", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), "qwen:0.5b", client.UserPrompt("why is the sky blue?"), ollama.WithChatStream(func(stream *ollama.Response) {
			t.Log(stream)
		}))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})

	t.Run("ChatNoStream", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), "qwen:0.5b", client.UserPrompt("why is the sky green?"))
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
	if err := client.PullModel(context.TODO(), "llama3.2:1b", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	})); err != nil {
		t.FailNow()
	}

	t.Run("Tools", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(), "llama3.2:1b",
			client.UserPrompt("what is the weather in berlin?"),
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

	// Delete model
	client.DeleteModel(context.TODO(), "llava")

	// Pull the model
	if err := client.PullModel(context.TODO(), "llava", ollama.WithPullStatus(func(status *ollama.PullStatus) {
		t.Log(status)
	})); err != nil {
		t.FailNow()
	}

	t.Run("Image", func(t *testing.T) {
		assert := assert.New(t)

		f, err := os.Open("testdata/guggenheim.jpg")
		if !assert.NoError(err) {
			t.FailNow()
		}
		defer f.Close()

		response, err := client.Chat(context.TODO(), "llava",
			client.UserPrompt("where was this photo taken?", ollama.WithData(f)),
		)
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})
}
