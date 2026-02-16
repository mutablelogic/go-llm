package agent_test

import (
	"encoding/json"
	"testing"

	// Packages
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_prepare_001(t *testing.T) {
	// Nil agent returns error
	assert := assert.New(t)
	_, err := agent.Prepare(nil, "", schema.GeneratorMeta{}, nil)
	assert.Error(err)
}

func Test_prepare_002(t *testing.T) {
	// Minimal agent with no template, no schema, no input
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name: "test",
		},
	}
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, nil)
	assert.NoError(err)
	assert.Equal("test", result.SessionMeta.Name)
	assert.Equal("", result.Text)
	assert.Equal("test@1", result.SessionMeta.Labels["agent"])
	assert.Empty(result.SessionMeta.Labels["parent"])
}

func Test_prepare_003(t *testing.T) {
	// Parent session ID is stored in labels
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 3,
		AgentMeta: schema.AgentMeta{
			Name: "myagent",
		},
	}
	result, err := agent.Prepare(a, "session-123", schema.GeneratorMeta{}, nil)
	assert.NoError(err)
	assert.Equal("myagent@3", result.SessionMeta.Labels["agent"])
	assert.Equal("session-123", result.SessionMeta.Labels["parent"])
}

func Test_prepare_004(t *testing.T) {
	// Defaults fill in blank GeneratorMeta fields
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name: "test",
			GeneratorMeta: schema.GeneratorMeta{
				Model: "agent-model",
			},
		},
	}
	defaults := schema.GeneratorMeta{
		Model:        "default-model",
		Provider:     "default-provider",
		SystemPrompt: "default-prompt",
	}
	result, err := agent.Prepare(a, "", defaults, nil)
	assert.NoError(err)
	assert.Equal("agent-model", result.SessionMeta.Model)           // agent wins
	assert.Equal("default-provider", result.SessionMeta.Provider)   // default fills in
	assert.Equal("default-prompt", result.SessionMeta.SystemPrompt) // default fills in
}

func Test_prepare_005(t *testing.T) {
	// Agent GeneratorMeta fields take precedence over defaults
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name: "test",
			GeneratorMeta: schema.GeneratorMeta{
				Model:        "agent-model",
				Provider:     "agent-provider",
				SystemPrompt: "agent-prompt",
			},
		},
	}
	defaults := schema.GeneratorMeta{
		Model:        "default-model",
		Provider:     "default-provider",
		SystemPrompt: "default-prompt",
	}
	result, err := agent.Prepare(a, "", defaults, nil)
	assert.NoError(err)
	assert.Equal("agent-model", result.SessionMeta.Model)
	assert.Equal("agent-provider", result.SessionMeta.Provider)
	assert.Equal("agent-prompt", result.SessionMeta.SystemPrompt)
}

func Test_prepare_006(t *testing.T) {
	// Template rendering with input
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:     "greeter",
			Template: "Hello, {{ .name }}!",
		},
	}
	input := json.RawMessage(`{"name": "World"}`)
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, input)
	assert.NoError(err)
	assert.Equal("Hello, World!", result.Text)
}

func Test_prepare_007(t *testing.T) {
	// Template with no input — template uses empty context
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:     "static",
			Template: "Static message",
		},
	}
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, nil)
	assert.NoError(err)
	assert.Equal("Static message", result.Text)
}

func Test_prepare_008(t *testing.T) {
	// Invalid JSON input returns error
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name: "test",
		},
	}
	input := json.RawMessage(`{invalid`)
	_, err := agent.Prepare(a, "", schema.GeneratorMeta{}, input)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid JSON")
}

func Test_prepare_009(t *testing.T) {
	// Input validation against schema — valid input
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:     "translate",
			Template: "Translate: {{ .text }}",
			Input: schema.NewJSONSchema(json.RawMessage(`{
				"type": "object",
				"properties": {
					"text": {"type": "string"},
					"target_language": {"type": "string"}
				},
				"required": ["text", "target_language"]
			}`)),
		},
	}
	input := json.RawMessage(`{"text": "Hello", "target_language": "French"}`)
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, input)
	assert.NoError(err)
	assert.Equal("Translate: Hello", result.Text)
}

func Test_prepare_010(t *testing.T) {
	// Input validation against schema — missing required field
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name: "translate",
			Input: schema.NewJSONSchema(json.RawMessage(`{
				"type": "object",
				"properties": {
					"text": {"type": "string"},
					"target_language": {"type": "string"}
				},
				"required": ["text", "target_language"]
			}`)),
		},
	}
	input := json.RawMessage(`{"text": "Hello"}`)
	_, err := agent.Prepare(a, "", schema.GeneratorMeta{}, input)
	assert.Error(err)
	assert.Contains(err.Error(), "input validation")
}

func Test_prepare_011(t *testing.T) {
	// Input validation — no input when schema requires fields
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name: "test",
			Input: schema.NewJSONSchema(json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string"}
				},
				"required": ["query"]
			}`)),
		},
	}
	_, err := agent.Prepare(a, "", schema.GeneratorMeta{}, nil)
	assert.Error(err)
	assert.Contains(err.Error(), "input validation")
}

func Test_prepare_012(t *testing.T) {
	// No schema — any input accepted
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:     "open",
			Template: "Got: {{ .anything }}",
		},
	}
	input := json.RawMessage(`{"anything": "works"}`)
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, input)
	assert.NoError(err)
	assert.Equal("Got: works", result.Text)
}

func Test_prepare_013(t *testing.T) {
	// Invalid template returns error
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:     "bad",
			Template: "{{ .foo",
		},
	}
	_, err := agent.Prepare(a, "", schema.GeneratorMeta{}, nil)
	assert.Error(err)
	assert.Contains(err.Error(), "template")
}

func Test_prepare_014(t *testing.T) {
	// Tools are passed through
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:  "tooled",
			Tools: []string{"weather", "search"},
		},
	}
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, nil)
	assert.NoError(err)
	assert.Equal([]string{"weather", "search"}, result.Tools)
}

func Test_prepare_015(t *testing.T) {
	// Template functions: json
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:     "jsontest",
			Template: `Data: {{ json . }}`,
		},
	}
	input := json.RawMessage(`{"key": "value"}`)
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, input)
	assert.NoError(err)
	assert.Contains(result.Text, `"key":"value"`)
}

func Test_prepare_016(t *testing.T) {
	// Template functions: upper, lower, trim
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:     "strtest",
			Template: `{{ upper .a }} {{ lower .b }} {{ trim .c }}`,
		},
	}
	input := json.RawMessage(`{"a": "hello", "b": "WORLD", "c": "  spaced  "}`)
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, input)
	assert.NoError(err)
	assert.Equal("HELLO world spaced", result.Text)
}

func Test_prepare_017(t *testing.T) {
	// Template function: default
	assert := assert.New(t)
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name:     "deftest",
			Template: `{{ default "fallback" .missing }}`,
		},
	}
	result, err := agent.Prepare(a, "", schema.GeneratorMeta{}, json.RawMessage(`{}`))
	assert.NoError(err)
	assert.Equal("fallback", result.Text)
}

func Test_prepare_018(t *testing.T) {
	// Thinking fields merge from defaults
	assert := assert.New(t)
	thinking := true
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name: "test",
		},
	}
	defaults := schema.GeneratorMeta{
		Thinking:       &thinking,
		ThinkingBudget: 1000,
	}
	result, err := agent.Prepare(a, "", defaults, nil)
	assert.NoError(err)
	assert.NotNil(result.SessionMeta.Thinking)
	assert.True(*result.SessionMeta.Thinking)
	assert.Equal(uint(1000), result.SessionMeta.ThinkingBudget)
}

func Test_prepare_019(t *testing.T) {
	// Agent thinking fields take precedence over defaults
	assert := assert.New(t)
	agentThinking := false
	defaultThinking := true
	a := &schema.Agent{
		Version: 1,
		AgentMeta: schema.AgentMeta{
			Name: "test",
			GeneratorMeta: schema.GeneratorMeta{
				Thinking:       &agentThinking,
				ThinkingBudget: 500,
			},
		},
	}
	defaults := schema.GeneratorMeta{
		Thinking:       &defaultThinking,
		ThinkingBudget: 2000,
	}
	result, err := agent.Prepare(a, "", defaults, nil)
	assert.NoError(err)
	assert.NotNil(result.SessionMeta.Thinking)
	assert.False(*result.SessionMeta.Thinking)
	assert.Equal(uint(500), result.SessionMeta.ThinkingBudget)
}

func Test_prepare_020(t *testing.T) {
	// Full end-to-end: ReadFile + Prepare
	assert := assert.New(t)
	meta, err := agent.ReadFile("../../etc/agent/translate.md")
	assert.NoError(err)

	a := &schema.Agent{
		ID:        "test-id",
		Version:   2,
		AgentMeta: meta,
	}
	defaults := schema.GeneratorMeta{
		Provider: "anthropic",
		Model:    "claude-haiku-4-5-20251001",
	}
	input := json.RawMessage(`{"text": "Hello", "target_language": "French"}`)
	result, err := agent.Prepare(a, "parent-sess", defaults, input)
	assert.NoError(err)
	assert.Equal("translate", result.SessionMeta.Name)
	assert.Equal("translate@2", result.SessionMeta.Labels["agent"])
	assert.Equal("parent-sess", result.SessionMeta.Labels["parent"])
	assert.Equal("anthropic", result.SessionMeta.Provider) // default fills in
	assert.Contains(result.Text, "Hello")
	assert.Contains(result.Text, "French")
}
