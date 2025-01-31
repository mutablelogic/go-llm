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

func Test_session_001(t *testing.T) {
	client, err := ollama.New(GetEndpoint(t), opts.OptTrace(os.Stderr, true))
	if err != nil {
		t.FailNow()
	}

	// Pull the model
	model, err := client.PullModel(context.TODO(), "qwen:0.5b")
	if err != nil {
		t.FailNow()
	}

	// Session with a single user prompt - streaming
	t.Run("stream", func(t *testing.T) {
		assert := assert.New(t)
		session := model.Context(ollama.WithStream(func(stream *ollama.Response) {
			t.Log("SESSION DELTA", stream)
		}))
		assert.NotNil(session)

		new_session, err := session.FromUser(context.TODO(), "Why is the grass green?")
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.Equal("assistant", new_session.Role())
		assert.NotEmpty(new_session.Text())
	})

	// Session with a single user prompt - not streaming
	t.Run("nostream", func(t *testing.T) {
		assert := assert.New(t)
		session := model.Context()
		assert.NotNil(session)

		new_session, err := session.FromUser(context.TODO(), "Why is the sky blue?")
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.Equal("assistant", new_session.Role())
		assert.NotEmpty(new_session.Text())
	})
}

func Test_session_002(t *testing.T) {
	client, err := ollama.New(GetEndpoint(t), opts.OptTrace(os.Stderr, true))
	if err != nil {
		t.FailNow()
	}

	// Pull the model
	model, err := client.PullModel(context.TODO(), "llama3.2")
	if err != nil {
		t.FailNow()
	}

	// Session with a tool call
	t.Run("toolcall", func(t *testing.T) {
		assert := assert.New(t)

		tool, err := ollama.NewTool("get_weather", "Return the current weather", nil)
		if !assert.NoError(err) {
			t.FailNow()
		}

		session := model.Context(ollama.WithTool(tool))
		assert.NotNil(session)
		new_session, err := session.FromUser(context.TODO(), "What is today's weather?")
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(new_session)
	})
}
