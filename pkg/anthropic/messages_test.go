package anthropic_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	assert "github.com/stretchr/testify/assert"
)

func Test_messages_001(t *testing.T) {
	assert := assert.New(t)
	client, err := anthropic.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	}

	model, err := client.GetModel(context.TODO(), "claude-3-haiku-20240307")
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	} else {
		t.FailNow()
	}

	f, err := os.Open("testdata/guggenheim.jpg")
	if !assert.NoError(err) {
		t.FailNow()
	}
	defer f.Close()

	response, err := client.Messages(context.TODO(), model.UserPrompt("what is this image?", anthropic.WithData(f, false, false)))
	if assert.NoError(err) {
		t.Log(response)
	}
}

func Test_messages_002(t *testing.T) {
	assert := assert.New(t)
	client, err := anthropic.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	}

	model, err := client.GetModel(context.TODO(), "claude-3-haiku-20240307")
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	} else {
		t.FailNow()
	}

	f, err := os.Open("testdata/LICENSE")
	if !assert.NoError(err) {
		t.FailNow()
	}
	defer f.Close()

	response, err := client.Messages(context.TODO(), model.UserPrompt("summarize this document for me", anthropic.WithData(f, false, false)))
	if assert.NoError(err) {
		t.Log(response)
	}
}

func Test_messages_003(t *testing.T) {
	assert := assert.New(t)
	client, err := anthropic.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	}

	model, err := client.GetModel(context.TODO(), "claude-3-haiku-20240307")
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	} else {
		t.FailNow()
	}

	response, err := client.Messages(context.TODO(), model.UserPrompt("why is the sky blue"), anthropic.WithStream(func(r *anthropic.Response) {
		t.Log(r)
	}))
	if assert.NoError(err) {
		t.Log(response)
	}
}

func Test_messages_004(t *testing.T) {
	assert := assert.New(t)
	client, err := anthropic.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	}

	model, err := client.GetModel(context.TODO(), "claude-3-haiku-20240307")
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	} else {
		t.FailNow()
	}

	weather, err := anthropic.NewTool("weather_in_location", "Get the weather in a location", struct {
		Location string `name:"location" help:"The location to get the weather for" required:"true"`
	}{})
	if !assert.NoError(err) {
		t.FailNow()
	}

	response, err := client.Messages(context.TODO(), model.UserPrompt("why is the sky blue"), anthropic.WithTool(weather))
	if assert.NoError(err) {
		t.Log(response)
	}
}

func Test_messages_005(t *testing.T) {
	assert := assert.New(t)
	client, err := anthropic.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	}

	model, err := client.GetModel(context.TODO(), "claude-3-haiku-20240307")
	if assert.NoError(err) {
		assert.NotNil(client)
		t.Log(client)
	} else {
		t.FailNow()
	}

	weather, err := anthropic.NewTool("weather_in_location", "Get the weather in a location", struct {
		Location string `name:"location" help:"The location to get the weather for" required:"true"`
	}{})
	if !assert.NoError(err) {
		t.FailNow()
	}

	response, err := client.Messages(context.TODO(), model.UserPrompt("why is the sky blue"), anthropic.WithStream(func(r *anthropic.Response) {
		t.Log(r)
	}), anthropic.WithTool(weather))
	if assert.NoError(err) {
		t.Log(response)
	}
}
