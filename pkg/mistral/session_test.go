package mistral_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-llm"
	mistral "github.com/mutablelogic/go-llm/pkg/mistral"
	"github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

func Test_session_001(t *testing.T) {
	assert := assert.New(t)

	client, err := mistral.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	model := client.Model(context.TODO(), "mistral-small-latest")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	session := model.Context()
	if assert.NotNil(session) {
		err := session.FromUser(context.TODO(), "Hello, how are you?")
		assert.NoError(err)
		t.Log(session)
	}
}

func Test_session_002(t *testing.T) {
	assert := assert.New(t)

	client, err := mistral.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	model := client.Model(context.TODO(), "mistral-small-latest")
	if !assert.NotNil(model) {
		t.FailNow()
	}

	toolkit := tool.NewToolKit()
	toolkit.Register(&weather{})

	session := model.Context(llm.WithToolKit(toolkit))
	if !assert.NotNil(session) {
		t.FailNow()
	}

	assert.NoError(session.FromUser(context.TODO(), "What is the weather like in London today?"))
	calls := session.ToolCalls(0)
	assert.Len(calls, 1)
	assert.Equal("weather_in_city", calls[0].Name())

	result, err := toolkit.Run(context.TODO(), calls...)
	assert.NoError(err)
	assert.Len(result, 1)

	assert.NoError(session.FromTool(context.TODO(), result...))

	t.Log(session)
}
