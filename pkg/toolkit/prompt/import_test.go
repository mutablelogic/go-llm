package prompt_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	// Packages
	prompt "github.com/mutablelogic/go-llm/pkg/toolkit/prompt"
	assert "github.com/stretchr/testify/assert"
)

func openFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func Test_Read_001(t *testing.T) {
	// no_frontmatter.md: name inferred from filename, template is full body
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/no_frontmatter.md"))
	assert.NoError(err)
	assert.NotNil(p)
	assert.Equal("no_frontmatter", p.Name())
	assert.Equal("No Frontmatter Agent", p.Title())
	assert.Equal("", p.Description())
}

func Test_Read_002(t *testing.T) {
	// minimal.md: name and title from front matter, empty template
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/minimal.md"))
	assert.NoError(err)
	assert.NotNil(p)
	assert.Equal("minimal", p.Name())
	assert.Equal("Minimal Agent", p.Title())
	assert.Equal("", p.Description())
}

func Test_Read_003(t *testing.T) {
	// summarizer.md: full front matter with generator hints, template body
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/summarizer.md"))
	assert.NoError(err)
	assert.NotNil(p)
	assert.Equal("summarizer", p.Name())
	assert.Equal("Text Summarizer", p.Title())
	assert.Equal("Summarizes input text into a concise paragraph", p.Description())
}

func Test_Read_004(t *testing.T) {
	// caption.md: uses output (format) field and provider/model hints
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/caption.md"))
	assert.NoError(err)
	assert.NotNil(p)
	assert.Equal("caption", p.Name())
	assert.Equal("Generate a caption from an attachment", p.Title())
}

func Test_Read_005(t *testing.T) {
	// classifier.md: thinking and thinking_budget fields
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/classifier.md"))
	assert.NoError(err)
	assert.NotNil(p)
	assert.Equal("classifier", p.Name())
	assert.Equal("Sentiment Classifier", p.Title())
}

func Test_Read_006(t *testing.T) {
	// bad_schema.md: output schema is missing "type" field — must return an error
	assert := assert.New(t)
	_, err := prompt.Read(openFile(t, "testdata/bad_schema.md"))
	assert.Error(err)
	assert.Contains(err.Error(), "output")
}

func Test_Read_007(t *testing.T) {
	// WithNamespace prefixes the name and delegates other methods
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/minimal.md"))
	assert.NoError(err)
	np := prompt.WithNamespace("myns", p)
	assert.Equal("myns.minimal", np.Name())
	assert.Equal(p.Title(), np.Title())
	assert.Equal(p.Description(), np.Description())
}

func Test_Read_008(t *testing.T) {
	// unclosed_fm.md: starts with --- but no closing --- so front matter is never
	// parsed; name comes from filename, template is empty
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/unclosed_fm.md"))
	assert.NoError(err)
	assert.NotNil(p)
	assert.Equal("unclosed_fm", p.Name())
	assert.Equal("", p.Title())
}

func Test_Read_009(t *testing.T) {
	// bad_input_schema.md: input schema is a JSON string not an object — FromJSON errors
	assert := assert.New(t)
	_, err := prompt.Read(openFile(t, "testdata/bad_input_schema.md"))
	assert.Error(err)
	assert.Contains(err.Error(), "input")
}

func Test_Read_010(t *testing.T) {
	// reader with no Name() method and no front-matter name → name is empty → error
	assert := assert.New(t)
	_, err := prompt.Read(strings.NewReader("hello world"))
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// MarshalJSON tests

func Test_JSON_001(t *testing.T) {
	// no_frontmatter.md: title comes from H1, no arguments → JSON has title but no arguments key
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/no_frontmatter.md"))
	assert.NoError(err)
	data, err := json.Marshal(p)
	assert.NoError(err)

	var got map[string]any
	assert.NoError(json.Unmarshal(data, &got))
	assert.Equal("no_frontmatter", got["name"])
	assert.Equal("No Frontmatter Agent", got["title"])
	assert.Nil(got["arguments"], "no arguments expected for a prompt with no input schema")
}

func Test_JSON_002(t *testing.T) {
	// minimal.md: title from front matter, no input schema → no arguments key
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/minimal.md"))
	assert.NoError(err)
	data, err := json.Marshal(p)
	assert.NoError(err)

	var got map[string]any
	assert.NoError(json.Unmarshal(data, &got))
	assert.Equal("minimal", got["name"])
	assert.Equal("Minimal Agent", got["title"])
	assert.Nil(got["arguments"])
}

func Test_JSON_003(t *testing.T) {
	// summarizer.md: one required "text" argument with description
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/summarizer.md"))
	assert.NoError(err)
	data, err := json.Marshal(p)
	assert.NoError(err)

	type arg struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
	}
	var got struct {
		Name      string `json:"name"`
		Title     string `json:"title"`
		Arguments []arg  `json:"arguments"`
	}
	assert.NoError(json.Unmarshal(data, &got))
	assert.Equal("summarizer", got.Name)
	assert.Equal("Text Summarizer", got.Title)
	if assert.Len(got.Arguments, 1) {
		assert.Equal("text", got.Arguments[0].Name)
		assert.Equal("The text to summarize", got.Arguments[0].Description)
		assert.True(got.Arguments[0].Required)
	}
}

func Test_JSON_004(t *testing.T) {
	// multi_args.md: arguments sorted alphabetically, required flag correct,
	// description preferred over title, title used as fallback when description absent
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/multi_args.md"))
	assert.NoError(err)
	data, err := json.Marshal(p)
	assert.NoError(err)

	type arg struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
	}
	var got struct {
		Arguments []arg `json:"arguments"`
	}
	assert.NoError(json.Unmarshal(data, &got))
	if assert.Len(got.Arguments, 3) {
		// Sorted: apple, mango, zebra
		assert.Equal("apple", got.Arguments[0].Name)
		assert.Equal("Apple description", got.Arguments[0].Description)
		assert.True(got.Arguments[0].Required)

		assert.Equal("mango", got.Arguments[1].Name)
		assert.Equal("Mango description", got.Arguments[1].Description, "description should win over title")
		assert.False(got.Arguments[1].Required)

		assert.Equal("zebra", got.Arguments[2].Name)
		assert.Equal("Zebra title", got.Arguments[2].Description, "title used as fallback when description absent")
		assert.False(got.Arguments[2].Required)
	}
}

func Test_JSON_005(t *testing.T) {
	// WithNamespace: namespaced name appears in marshaled JSON, other fields unchanged
	assert := assert.New(t)
	p, err := prompt.Read(openFile(t, "testdata/minimal.md"))
	assert.NoError(err)
	np := prompt.WithNamespace("myns", p)
	data, err := json.Marshal(np)
	assert.NoError(err)

	var got map[string]any
	assert.NoError(json.Unmarshal(data, &got))
	assert.Equal("myns.minimal", got["name"])
	assert.Equal("Minimal Agent", got["title"])
}
