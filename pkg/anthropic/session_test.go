package anthropic_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

func Test_session_001(t *testing.T) {
	client, err := anthropic.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	if err != nil {
		t.FailNow()
	}

	model, err := client.GetModel(context.TODO(), "claude-3-haiku-20240307")
	if err != nil {
		t.FailNow()
	}

	// Session with a single user prompt - streaming
	t.Run("stream", func(t *testing.T) {
		assert := assert.New(t)
		session := model.Context(llm.WithStream(func(stream llm.ContextContent) {
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
	client, err := anthropic.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	if err != nil {
		t.FailNow()
	}

	model, err := client.GetModel(context.TODO(), "claude-3-haiku-20240307")
	if err != nil {
		t.FailNow()
	}

	// Session with a tool call
	t.Run("toolcall", func(t *testing.T) {
		assert := assert.New(t)

		toolkit := tool.NewToolKit()
		if err := toolkit.Register(new(weather)); !assert.NoError(err) {
			t.FailNow()
		}

		session := model.Context(llm.WithToolKit(toolkit))
		assert.NotNil(session)

		err = session.FromUser(context.TODO(), "What is today's weather, in Berlin?")
		if !assert.NoError(err) {
			t.FailNow()
		}

		err := toolkit.Run(context.TODO(), session.ToolCalls()...)
		if !assert.NoError(err) {
			t.FailNow()
		}
	})
}
