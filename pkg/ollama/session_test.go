package ollama_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	"github.com/mutablelogic/go-llm/pkg/tool"
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
		session := model.Context(llm.WithStream(func(stream llm.Context) {
			t.Log("SESSION DELTA", stream)
		}))
		assert.NotNil(session)

		err := session.FromUser(context.TODO(), "Why is the grass green?")
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.Equal("assistant", session.Role())
		assert.NotEmpty(session.Text())
	})

	// Session with a single user prompt - not streaming
	t.Run("nostream", func(t *testing.T) {
		assert := assert.New(t)
		session := model.Context()
		assert.NotNil(session)

		err := session.FromUser(context.TODO(), "Why is the sky blue?")
		if !assert.NoError(err) {
			t.FailNow()
		}
		assert.Equal("assistant", session.Role())
		assert.NotEmpty(session.Text())
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

	// Make a toolkit
	toolkit := tool.NewToolKit()
	if err := toolkit.Register(new(weather)); err != nil {
		t.FailNow()
	}

	// Session with a tool call
	t.Run("toolcall", func(t *testing.T) {
		assert := assert.New(t)

		session := model.Context(llm.WithToolKit(toolkit))
		assert.NotNil(session)

		err = session.FromUser(context.TODO(), "What is today's weather in Berlin?", llm.WithTemperature(0.5))
		if !assert.NoError(err) {
			t.FailNow()
		}
		t.Log(session)
	})
}
