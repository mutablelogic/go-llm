package anthropic_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	"github.com/mutablelogic/go-llm/pkg/tool"
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

	toolkit := tool.NewToolKit()
	if err := toolkit.Register(new(weather)); !assert.NoError(err) {
		t.FailNow()
	}

	response, err := client.Messages(context.TODO(), model.UserPrompt("why is the sky blue"), anthropic.WithToolKit(toolkit))
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

	toolkit := tool.NewToolKit()
	if err := toolkit.Register(new(weather)); !assert.NoError(err) {
		t.FailNow()
	}

	response, err := client.Messages(context.TODO(), model.UserPrompt("why is the sky blue"), anthropic.WithStream(func(r *anthropic.Response) {
		t.Log(r)
	}), anthropic.WithToolKit(toolkit))
	if assert.NoError(err) {
		t.Log(response)
	}
}

////////////////////////////////////////////////////////////////////////////////
// TOOLS

type weather struct {
	Location string `name:"location" help:"The location to get the weather for" required:"true"`
}

func (*weather) Name() string {
	return "weather_in_location"
}

func (*weather) Description() string {
	return "Get the weather in a location"
}

func (*weather) Run(ctx context.Context) (any, error) {
	return nil, llm.ErrNotImplemented
}
