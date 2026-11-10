package namespace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// RunFunc runs the agent with the name, in its namespace, such as
// github:review_pr, with the input, as a tool which a model called, as the
// registry runs its agents, and returns its reply
type RunFunc func(ctx context.Context, name string, input json.RawMessage) (any, error)

// prompt is one of an MCP server's prompts, as an agent, which is a tool, by
// its name, as an identifier, such as args_prompt for args-prompt, which is
// run as the registry runs its agents
type prompt struct {
	toolkit.Base
	p         *sdkmcp.Prompt
	name      string
	namespace string
	input     *jsonschema.Schema
	run       RunFunc
}

var _ llm.Tool = (*prompt)(nil)
var _ toolkit.AgentTool = (*prompt)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newPrompt returns the server's prompt, as an agent, in the namespace, by
// the name, with the input of its arguments, each of which is a string,
// which is run with the function
func newPrompt(p *sdkmcp.Prompt, namespace, name string, run RunFunc) *prompt {
	return &prompt{p: p, name: name, namespace: namespace, input: promptInput(p), run: run}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the prompt's name, as an identifier
func (p *prompt) Name() string { return p.name }

// Title returns the prompt's title, for people, or none
func (p *prompt) Title() string { return strings.TrimSpace(p.p.Title) }

// promptAgent is what a model which calls a prompt is told it is, so it
// expects the reply of the model which is asked it, rather than its text
const promptAgent = "This is an agent: it asks a model the server's prompt, with the input, and returns the model's reply, rather than the prompt's text. " + toolkit.AgentCall

// Description returns what the prompt is for, and that it's an agent
func (p *prompt) Description() string {
	if description := firstOf(p.p.Description, p.Title()); description != "" {
		return strings.TrimRight(description, ". ") + ". " + promptAgent
	}
	return promptAgent
}

// InputSchema returns the schema of the prompt's arguments, as an object of
// strings
func (p *prompt) InputSchema() *jsonschema.Schema { return p.input }

// IsAgent returns true, as a prompt is run as an agent
func (*prompt) IsAgent() bool { return true }

// Run runs the prompt, as an agent, with the input, as the registry runs
// its agents
func (p *prompt) Run(ctx context.Context, input json.RawMessage) (any, error) {
	return p.run(ctx, p.namespace+":"+p.name, input)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// promptInput returns the schema of the prompt's arguments: an object, of
// strings, which are required, or not, as the prompt says, and nothing else
func promptInput(p *sdkmcp.Prompt) *jsonschema.Schema {
	properties := map[string]any{}
	required := []string{}
	for _, argument := range p.Arguments {
		if argument == nil || argument.Name == "" {
			continue
		}
		property := map[string]any{"type": "string"}
		if description := firstOf(argument.Description, argument.Title); description != "" {
			property["description"] = description
		}
		properties[argument.Name] = property
		if argument.Required {
			required = append(required, argument.Name)
		}
	}
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schemaOf(schema)
}

// promptArguments returns the input, as the prompt's arguments, which are
// strings, of which a value which isn't one is its JSON, or ErrBadParameter
// when it isn't an object, or one which is required isn't given, whatever
// its value, which is the server's to judge
func promptArguments(p *sdkmcp.Prompt, input json.RawMessage) (map[string]string, error) {
	values := map[string]any{}
	if text := strings.TrimSpace(string(input)); text != "" && text != "null" {
		if err := json.Unmarshal(input, &values); err != nil {
			return nil, llm.ErrBadParameter.Withf("the input of prompt %q isn't an object: %v", p.Name, err)
		}
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		switch value := value.(type) {
		case nil:
		case string:
			result[key] = value
		default:
			data, err := json.Marshal(value)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("argument %q of prompt %q: %v", key, p.Name, err)
			}
			result[key] = string(data)
		}
	}
	for _, argument := range p.Arguments {
		if _, given := result[argument.Name]; argument != nil && argument.Required && !given {
			return nil, llm.ErrBadParameter.Withf("prompt %q needs argument %q", p.Name, argument.Name)
		}
	}
	return result, nil
}

// promptMessages returns the prompt's messages, as a model is sent them:
// text as it is, an embedded resource's text, under its URI, its data, an
// image's, or audio's, as an attachment, and a link to a resource as text,
// which says what it is
func promptMessages(result *sdkmcp.GetPromptResult) []llm.Message {
	messages := make([]llm.Message, 0, len(result.Messages))
	for _, m := range result.Messages {
		if m == nil || m.Content == nil {
			continue
		}
		role := "user"
		if m.Role == "assistant" {
			role = "assistant"
		}
		var block providerschema.ContentBlock
		switch content := m.Content.(type) {
		case *sdkmcp.TextContent:
			block.Text = types.Ptr(content.Text)
		case *sdkmcp.ImageContent:
			block.Resource = &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: content.MIMEType, Data: content.Data}}
		case *sdkmcp.AudioContent:
			block.Resource = &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: content.MIMEType, Data: content.Data}}
		case *sdkmcp.EmbeddedResource:
			if resource := content.Resource; resource != nil && resource.Text != "" {
				block = providerschema.TextBlock(resource.URI, []byte(resource.Text))
			} else if resource != nil && len(resource.Blob) > 0 {
				block.Resource = &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: resource.MIMEType, Data: resource.Blob}}
			}
		case *sdkmcp.ResourceLink:
			block.Text = types.Ptr(fmt.Sprintf("(A resource, %s, at %s, which isn't read.)", firstOf(content.Title, content.Name), content.URI))
		}
		if block.Text == nil && block.Resource == nil {
			continue
		}
		messages = append(messages, &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: role, Content: []providerschema.ContentBlock{block}}})
	}
	return messages
}

// promptName returns the prompt's name as an identifier, which a tool's name
// is, of letters, digits and underscores, starting with a letter, without
// two underscores together, or one at its end, such as args_prompt for
// args-prompt, or empty, when nothing's left of it
func promptName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9' && b.Len() > 0:
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "_"):
			b.WriteRune('_')
		}
	}
	return strings.TrimRight(b.String(), "_")
}

// firstOf returns the first value which isn't empty
func firstOf(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
