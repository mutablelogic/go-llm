package cmd

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestAskCommandRequestWithFileAttachments(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()
	first := filepath.Join(dir, "alpha.txt")
	second := filepath.Join(dir, "beta.txt")
	require.NoError(t, os.WriteFile(first, []byte("alpha"), 0o600))
	require.NoError(t, os.WriteFile(second, []byte("beta"), 0o600))

	req, err := (AskCommand{
		GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("phi4:latest"), Provider: types.Ptr("ollama")},
		Text:          "summarize these files",
		File:          []string{filepath.Join(dir, "*.txt")},
	}).request()
	if !assert.NoError(err) {
		return
	}

	assert.Equal(types.Ptr("phi4:latest"), req.Model)
	assert.Equal(types.Ptr("ollama"), req.Provider)
	assert.Equal("summarize these files", req.Text)
	if assert.Len(req.Attachments, 2) {
		assert.Equal("text/plain; charset=utf-8", req.Attachments[0].ContentType)
		assert.Equal([]byte("alpha"), req.Attachments[0].Data)
		if assert.NotNil(req.Attachments[0].URL) {
			assert.Equal("file", req.Attachments[0].URL.Scheme)
			assert.Equal(first, req.Attachments[0].URL.Path)
		}

		assert.Equal("text/plain; charset=utf-8", req.Attachments[1].ContentType)
		assert.Equal([]byte("beta"), req.Attachments[1].Data)
		if assert.NotNil(req.Attachments[1].URL) {
			assert.Equal("file", req.Attachments[1].URL.Scheme)
			assert.Equal(second, req.Attachments[1].URL.Path)
		}
	}
}

func TestAskAttachmentsNoMatches(t *testing.T) {
	assert := assert.New(t)
	_, err := askAttachments([]string{"/definitely/not/here/*.txt"})
	if assert.Error(err) {
		assert.Contains(err.Error(), "no files match")
	}
}

func TestAskAttachmentsInvalidGlob(t *testing.T) {
	assert := assert.New(t)
	_, err := askAttachments([]string{"["})
	if assert.Error(err) {
		assert.Contains(err.Error(), "invalid glob pattern")
	}
}

func TestAskResponseAttachmentURLUsesOutputDirAndBasename(t *testing.T) {
	assert := assert.New(t)
	out := t.TempDir()
	attachment := &schema.Attachment{
		ContentType: "image/png",
		URL:         &url.URL{Scheme: "https", Host: "example.com", Path: "/nested/cat.png"},
	}

	target, err := askResponseAttachmentURL(attachment, out, 0)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("file", target.Scheme)
	assert.Equal(filepath.Join(out, "cat.png"), target.Path)
}

func TestAskResponseAttachmentURLGeneratesFilenameFromContentType(t *testing.T) {
	assert := assert.New(t)
	out := t.TempDir()
	attachment := &schema.Attachment{ContentType: "image/png"}

	target, err := askResponseAttachmentURL(attachment, out, 1)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("file", target.Scheme)
	assert.Equal(filepath.Join(out, "attachment-002.png"), target.Path)
}

func TestWriteAskResponseAttachmentWritesFileAndAvoidsCollisions(t *testing.T) {
	assert := assert.New(t)
	out := t.TempDir()
	first := &schema.Attachment{
		ContentType: "text/plain; charset=utf-8",
		Data:        []byte("alpha"),
		URL:         &url.URL{Scheme: "file", Path: "/tmp/report.txt"},
	}
	second := &schema.Attachment{
		ContentType: "text/plain; charset=utf-8",
		Data:        []byte("beta"),
		URL:         &url.URL{Scheme: "file", Path: "/tmp/report.txt"},
	}

	firstTarget, err := writeAskResponseAttachment(first, out, 0)
	if !assert.NoError(err) {
		return
	}
	secondTarget, err := writeAskResponseAttachment(second, out, 1)
	if !assert.NoError(err) {
		return
	}

	assert.Equal(filepath.Join(out, "report.txt"), firstTarget.Path)
	assert.Equal(filepath.Join(out, "report-2.txt"), secondTarget.Path)

	firstData, err := os.ReadFile(firstTarget.Path)
	if !assert.NoError(err) {
		return
	}
	secondData, err := os.ReadFile(secondTarget.Path)
	if !assert.NoError(err) {
		return
	}

	assert.Equal([]byte("alpha"), firstData)
	assert.Equal([]byte("beta"), secondData)
	if assert.NotNil(first.URL) {
		assert.Equal(firstTarget.Path, first.URL.Path)
	}
	if assert.NotNil(second.URL) {
		assert.Equal(secondTarget.Path, second.URL.Path)
	}
}

func TestWriteMarkdown(t *testing.T) {
	assert := assert.New(t)
	widget := tui.Markdown(tui.SetWidth(40))
	var buffer bytes.Buffer

	err := writeMarkdown(&buffer, widget, "# Title\n\nA **bold** paragraph")
	if !assert.NoError(err) {
		return
	}

	out := buffer.String()
	assert.Contains(out, "Title")
	assert.Contains(out, "bold")
	assert.Contains(out, "\n")
}

func TestMarkdownStreamBuffersUntilFinish(t *testing.T) {
	assert := assert.New(t)
	widget := tui.Markdown(tui.SetWidth(40))
	var buffer bytes.Buffer
	stream := newMarkdownStream(&buffer, widget)

	assert.NoError(stream.Append("# Title"))
	assert.NoError(stream.Append("\n\nA **bold** paragraph\n\n"))
	assert.Contains(buffer.String(), "Title")
	assert.Contains(buffer.String(), "bold")

	if !assert.NoError(stream.Append("- One\n- Two")) {
		return
	}

	out := buffer.String()
	assert.Contains(out, "Title")
	assert.Contains(out, "bold")
	assert.NotContains(out, "One")

	if !assert.NoError(stream.Finish("")) {
		return
	}

	out = buffer.String()
	assert.Contains(out, "Title")
	assert.Contains(out, "bold")
	assert.Contains(out, "One")
	assert.Contains(out, "Two")
	assert.Contains(out, "\n")
}

func TestMarkdownStreamFinishDoesNotDuplicateFlushedContent(t *testing.T) {
	assert := assert.New(t)
	var buffer bytes.Buffer
	stream := newMarkdownStream(&buffer, passthroughMarkdownWidget{})
	full := "Alpha\n\nBeta\n\nGamma"

	if !assert.NoError(stream.Append(full)) {
		return
	}
	if !assert.NoError(stream.Finish(full + "\n\n- [Attachment 1](file:///tmp/result.txt)\n")) {
		return
	}

	out := buffer.String()
	assert.Equal(1, strings.Count(out, "Alpha"))
	assert.Equal(1, strings.Count(out, "Beta"))
	assert.Equal(1, strings.Count(out, "Gamma"))
	assert.Equal(1, strings.Count(out, "Attachment 1"))
}

func TestSplitMarkdownFlushable(t *testing.T) {
	assert := assert.New(t)

	flushable, pending := splitMarkdownFlushable("# Title\n\nParagraph\n\nNext")
	assert.Equal("# Title\n\nParagraph", flushable)
	assert.Equal("Next", strings.TrimSpace(pending))

	flushable, pending = splitMarkdownFlushable("```go\nfmt.Println(1)\n")
	assert.Equal("", flushable)
	assert.Equal("```go\nfmt.Println(1)\n", pending)
}

type passthroughMarkdownWidget struct{}

func (passthroughMarkdownWidget) Write(w io.Writer, text string) (int, error) {
	return io.WriteString(w, text)
}
