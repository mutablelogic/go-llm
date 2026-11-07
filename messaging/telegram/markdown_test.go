package telegram

import (
	"strings"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Markdown_Parts(t *testing.T) {
	html := func(markdown string) string {
		t.Helper()
		result := parts(markdown, maxText)
		require.Len(t, result, 1, markdown)
		return result[0].html
	}

	for markdown, want := range map[string]string{
		"Hello":                           "Hello",
		"**bold**, *italic* and ~~gone~~": "<b>bold</b>, <i>italic</i> and <s>gone</s>",
		"Use `a < b && c`":                "Use <code>a &lt; b &amp;&amp; c</code>",
		"# Weather\n\nSunny":              "<b>Weather</b>\n\nSunny",
		"[Berlin](https://example.com/?a=1&b=\"2\")": `<a href="https://example.com/?a=1&amp;b=&quot;2&quot;">Berlin</a>`,
		"See https://example.com today":              `See <a href="https://example.com">https://example.com</a> today`,
		"![a map](https://example.com/m.png)":        `<a href="https://example.com/m.png">a map</a>`,
		"> Quoted\n> **text**":                       "<blockquote>Quoted\n<b>text</b></blockquote>",
		"one\ntwo":                                   "one\ntwo",
		"---":                                        "———",
		"<b>raw</b> html":                            "&lt;b&gt;raw&lt;/b&gt; html",
		"```go\nfmt.Println(\"<hi>\")\n```":          "<pre><code class=\"language-go\">fmt.Println(\"&lt;hi&gt;\")</code></pre>",
		"- one\n- **two**\n  - nested\n- [x] done":   "• one\n• <b>two</b>\n    • nested\n• ☑ done",
		"3. three\n4. four":                          "3. three\n4. four",
	} {
		assert.Equal(t, want, html(markdown), markdown)
	}

	// A table is preformatted, with its columns lined up
	table := html("| City | Temp |\n|---|---|\n| Berlin | 4°C |\n| Lisbon | 18°C |")
	assert.Equal(t, "<pre>City   │ Temp\n───────┼─────\nBerlin │ 4°C\nLisbon │ 18°C</pre>", table)
}

func Test_Markdown_Split(t *testing.T) {
	// Paragraphs are kept together, up to the limit, and broken between
	paragraph := strings.TrimSpace(strings.Repeat("word ", 15)) // 74 characters
	result := parts(strings.Repeat("**"+paragraph+"**\n\n", 4), 200)
	require.Len(t, result, 2)
	for _, p := range result {
		assert.Equal(t, 2, strings.Count(p.html, "<b>"))
		assert.Equal(t, strings.Count(p.html, "<b>"), strings.Count(p.html, "</b>"))
		assert.NotContains(t, p.plain, "<b>")
	}

	// A block which is longer than a message is plain text, in parts
	result = parts("**"+strings.Repeat("long ", 100)+"** & more", 200)
	require.Greater(t, len(result), 1)
	for _, p := range result {
		assert.NotContains(t, p.html, "<b>")
		assert.LessOrEqual(t, len([]rune(p.html)), 200)
	}
	assert.Contains(t, result[len(result)-1].html, "&amp; more")
	assert.Contains(t, result[len(result)-1].plain, "& more")

	// Nothing is no messages
	assert.Empty(t, parts("  \n\n ", 200))
}
