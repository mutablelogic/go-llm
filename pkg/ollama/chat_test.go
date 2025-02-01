package ollama_test

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
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
		response, err := client.Chat(context.TODO(), model.UserPrompt("why is the sky blue?"), llm.WithStream(func(stream llm.Context) {
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

	// Make a toolkit
	toolkit := tool.NewToolKit()
	if err := toolkit.Register(new(weather)); err != nil {
		t.FailNow()
	}

	t.Run("Tools", func(t *testing.T) {
		assert := assert.New(t)
		response, err := client.Chat(context.TODO(),
			model.UserPrompt("what is the weather in berlin?"),
			llm.WithToolKit(toolkit),
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
			model.UserPrompt("describe this photo to me", llm.WithAttachment(f)),
		)
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(response)
	})
}

////////////////////////////////////////////////////////////////////////////////
// TOOLS

type weather struct {
	Location string `json:"location" name:"location" help:"The location to get the weather for" required:"true"`
}

func (*weather) Name() string {
	return "weather_in_location"
}

func (*weather) Description() string {
	return "Get the weather in a location"
}

func (weather *weather) String() string {
	data, err := json.MarshalIndent(weather, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (weather *weather) Run(ctx context.Context) (any, error) {
	log.Println("weather_in_location", "=>", weather)
	return "very sunny today", nil
}
