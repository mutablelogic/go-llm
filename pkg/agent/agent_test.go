package agent_test

import (
	"strings"
	"testing"

	agent "github.com/mutablelogic/go-llm/pkg/agent"
	assert "github.com/stretchr/testify/assert"
)

func Test_read_001(t *testing.T) {
	// Empty document — name is required
	assert := assert.New(t)
	_, err := agent.Read(strings.NewReader(""))
	assert.Error(err)
	assert.Contains(err.Error(), "name")
}

func Test_read_002(t *testing.T) {
	// Body only, no front matter — name is required
	assert := assert.New(t)
	_, err := agent.Read(strings.NewReader("Hello, world!"))
	assert.Error(err)
	assert.Contains(err.Error(), "name")
}

func Test_read_003(t *testing.T) {
	assert := assert.New(t)
	doc := "---\nname: test-agent\ntitle: Test Agent\n---\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal("test-agent", meta.Name)
	assert.Equal("Test Agent", meta.Title)
	assert.Empty(meta.SystemPrompt)
	assert.Empty(meta.Template)
}

func Test_read_003a(t *testing.T) {
	// Tools defaults to empty (not nil) when not specified
	assert := assert.New(t)
	doc := "---\nname: no-tools\ntitle: No Tools Agent\n---\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.NotNil(meta.Tools)
	assert.Empty(meta.Tools)
}

func Test_read_003b(t *testing.T) {
	// Explicit tools are preserved
	assert := assert.New(t)
	doc := "---\nname: with-tools\ntitle: With Tools Agent\ntools:\n  - search\n  - calculator\n---\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal([]string{"search", "calculator"}, meta.Tools)
}

func Test_read_003c(t *testing.T) {
	// Explicit empty tools list is preserved as empty (not nil)
	assert := assert.New(t)
	doc := "---\nname: empty-tools\ntitle: Empty Tools Agent\ntools: []\n---\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.NotNil(meta.Tools)
	assert.Empty(meta.Tools)
}

func Test_read_004(t *testing.T) {
	assert := assert.New(t)
	doc := "---\nname: summarizer\ntitle: Summarizer Agent\ndescription: Summarizes text input\nmodel: gemini-2.0-flash\nprovider: gemini\n---\nYou are a summarization agent.\n\nGiven the following text, provide a concise summary.\n\n{{ .Input }}\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal("summarizer", meta.Name)
	assert.Equal("Summarizer Agent", meta.Title)
	assert.Equal("Summarizes text input", meta.Description)
	assert.Equal("gemini-2.0-flash", meta.Model)
	assert.Equal("gemini", meta.Provider)
	assert.Contains(meta.Template, "You are a summarization agent.")
	assert.Contains(meta.Template, "{{ .Input }}")
}

func Test_read_005(t *testing.T) {
	assert := assert.New(t)
	doc := "---\nname: thinker\ntitle: Deep Thinker\nthinking: true\nthinking_budget: 4096\nmodel: claude-sonnet\nprovider: anthropic\n---\nThink carefully about this problem.\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal("thinker", meta.Name)
	assert.NotNil(meta.Thinking)
	assert.True(*meta.Thinking)
	assert.Equal(uint(4096), meta.ThinkingBudget)
}

func Test_read_007(t *testing.T) {
	assert := assert.New(t)
	doc := "---\n: invalid yaml [[\n---\n"
	_, err := agent.Read(strings.NewReader(doc))
	assert.Error(err)
}

func Test_read_008(t *testing.T) {
	assert := assert.New(t)
	doc := "---\nname: no-close\ntitle: Missing Close\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal("no-close", meta.Name)
	assert.Equal("Missing Close", meta.Title)
	assert.Empty(meta.SystemPrompt)
	assert.Empty(meta.Template)
}

func Test_read_009(t *testing.T) {
	// Valid format and input schemas (YAML objects with type field)
	assert := assert.New(t)
	doc := "---\nname: structured\ntitle: Structured Agent\nformat:\n  type: object\n  properties:\n    summary:\n      type: string\ninput:\n  type: object\n  properties:\n    text:\n      type: string\n  required:\n    - text\n---\nSummarize the input.\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal("structured", meta.Name)
	assert.NotNil(meta.Format)
	assert.Contains(string(meta.Format), `"summary"`)
	assert.NotNil(meta.Input)
	assert.Contains(string(meta.Input), `"text"`)
}

func Test_read_010(t *testing.T) {
	// Format missing "type" field
	assert := assert.New(t)
	doc := "---\nname: bad-format\ntitle: Bad Format Agent\nformat:\n  properties:\n    x:\n      type: string\n---\n"
	_, err := agent.Read(strings.NewReader(doc))
	assert.Error(err)
	assert.Contains(err.Error(), "format")
	assert.Contains(err.Error(), "type")
}

func Test_read_011(t *testing.T) {
	// Input missing "type" field
	assert := assert.New(t)
	doc := "---\nname: bad-input\ntitle: Bad Input Agent\ninput:\n  properties:\n    x:\n      type: string\n---\n"
	_, err := agent.Read(strings.NewReader(doc))
	assert.Error(err)
	assert.Contains(err.Error(), "input")
	assert.Contains(err.Error(), "type")
}

func Test_read_012(t *testing.T) {
	// Format as a non-object (scalar string)
	assert := assert.New(t)
	doc := "---\nname: bad-format-scalar\ntitle: Bad Format Scalar\nformat: not-an-object\n---\n"
	_, err := agent.Read(strings.NewReader(doc))
	assert.Error(err)
	assert.Contains(err.Error(), "format")
}

func Test_readfile_summarizer(t *testing.T) {
	assert := assert.New(t)
	meta, err := agent.ReadFile("testdata/summarizer.md")
	assert.NoError(err)
	assert.Equal("summarizer", meta.Name)
	assert.Equal("Text Summarizer", meta.Title)
	assert.Equal("Summarizes input text into a concise paragraph", meta.Description)
	assert.Equal("gemini-2.0-flash", meta.Model)
	assert.Equal("gemini", meta.Provider)
	assert.NotNil(meta.Format)
	assert.Contains(string(meta.Format), `"summary"`)
	assert.NotNil(meta.Input)
	assert.Contains(string(meta.Input), `"text"`)
	assert.Equal("You are a professional text summarizer.", meta.SystemPrompt)
	assert.Contains(meta.Template, "{{ .text }}")
	assert.NotContains(meta.Template, "You are a professional")
	assert.Nil(meta.Thinking)
}

func Test_readfile_classifier(t *testing.T) {
	assert := assert.New(t)
	meta, err := agent.ReadFile("testdata/classifier.md")
	assert.NoError(err)
	assert.Equal("classifier", meta.Name)
	assert.Equal("Sentiment Classifier", meta.Title)
	assert.Equal("claude-sonnet", meta.Model)
	assert.Equal("anthropic", meta.Provider)
	assert.NotNil(meta.Thinking)
	assert.True(*meta.Thinking)
	assert.Equal(uint(2048), meta.ThinkingBudget)
	assert.NotNil(meta.Format)
	assert.Contains(string(meta.Format), `"sentiment"`)
	assert.Contains(string(meta.Format), `"confidence"`)
	assert.Nil(meta.Input)
	assert.Equal("You are a sentiment analysis expert.", meta.SystemPrompt)
	assert.Contains(meta.Template, "sentiment")
	assert.NotContains(meta.Template, "You are a sentiment")
}

func Test_readfile_no_frontmatter(t *testing.T) {
	// No front matter — name from filename, title from heading
	assert := assert.New(t)
	meta, err := agent.ReadFile("testdata/no_frontmatter.md")
	assert.NoError(err)
	assert.Equal("no_frontmatter", meta.Name)
	assert.Equal("No Frontmatter Agent", meta.Title)
	assert.NotEmpty(meta.Template)
}

func Test_readfile_minimal(t *testing.T) {
	assert := assert.New(t)
	meta, err := agent.ReadFile("testdata/minimal.md")
	assert.NoError(err)
	assert.Equal("minimal", meta.Name)
	assert.Equal("gemini-2.0-flash", meta.Model)
	assert.Equal("Minimal Agent", meta.Title)
	assert.Empty(meta.Description)
	assert.Nil(meta.Format)
	assert.Nil(meta.Input)
	assert.Empty(meta.SystemPrompt)
	assert.Empty(meta.Template)
}

func Test_readfile_bad_schema(t *testing.T) {
	assert := assert.New(t)
	_, err := agent.ReadFile("testdata/bad_schema.md")
	assert.Error(err)
	assert.Contains(err.Error(), "format")
	assert.Contains(err.Error(), "type")
}

func Test_readfile_not_found(t *testing.T) {
	assert := assert.New(t)
	_, err := agent.ReadFile("testdata/nonexistent.md")
	assert.Error(err)
}

func Test_read_short_title(t *testing.T) {
	// Title too short (under 10 characters)
	assert := assert.New(t)
	doc := "---\nname: agent-x\ntitle: Short\n---\n"
	_, err := agent.Read(strings.NewReader(doc))
	assert.Error(err)
	assert.Contains(err.Error(), "title")
}

func Test_read_empty_title(t *testing.T) {
	// Title is empty
	assert := assert.New(t)
	doc := "---\nname: agent-x\n---\n"
	_, err := agent.Read(strings.NewReader(doc))
	assert.Error(err)
	assert.Contains(err.Error(), "title")
}

func Test_read_whitespace_title(t *testing.T) {
	// Title is only whitespace
	assert := assert.New(t)
	doc := "---\nname: agent-x\ntitle: \"   \"\n---\n"
	_, err := agent.Read(strings.NewReader(doc))
	assert.Error(err)
	assert.Contains(err.Error(), "title")
}

func Test_read_title_from_heading(t *testing.T) {
	// No title in front matter — extract from first # heading
	assert := assert.New(t)
	doc := "---\nname: agent-x\n---\n# My Awesome Agent\n\nSome body text.\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal("My Awesome Agent", meta.Title)
}

func Test_read_title_from_second_heading(t *testing.T) {
	// Only the first # heading is used
	assert := assert.New(t)
	doc := "---\nname: agent-x\n---\nSome preamble.\n\n# First Heading Here\n\n# Second Heading\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal("First Heading Here", meta.Title)
}

func Test_read_title_frontmatter_over_heading(t *testing.T) {
	// Front matter title takes precedence over heading
	assert := assert.New(t)
	doc := "---\nname: agent-x\ntitle: From Frontmatter\n---\n# From Heading\n"
	meta, err := agent.Read(strings.NewReader(doc))
	assert.NoError(err)
	assert.Equal("From Frontmatter", meta.Title)
}
