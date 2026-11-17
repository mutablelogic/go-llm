package workspace_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// text returns a resource of the type, with the data
func text(contentType, data string) llm.Resource {
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: contentType, Data: []byte(data)}}
}

func Test_ReadMarkdown(t *testing.T) {
	ctx := context.Background()

	t.Run("Text", func(t *testing.T) {
		part, err := workspace.ReadMarkdown(ctx, text("text/markdown", "# Note\n\nHello"), 0, 0)
		require.NoError(t, err)
		assert.Contains(t, part.Body, "Hello")
		assert.Nil(t, part.Next, "it's all of it")
	})

	t.Run("Parts", func(t *testing.T) {
		// A part at a time, carrying on where the last ended
		long := strings.Repeat("Hello, world. ", 200)
		first, err := workspace.ReadMarkdown(ctx, text("text/plain", long), 0, 100)
		require.NoError(t, err)
		require.NotNil(t, first.Next)
		assert.LessOrEqual(t, len(first.Body), 200)
		second, err := workspace.ReadMarkdown(ctx, text("text/plain", long), *first.Next, 100)
		require.NoError(t, err)
		assert.Equal(t, *first.Next, second.Offset)

		// The first part has the length of all of the Markdown, which the
		// offsets are of, as the last does
		require.NotNil(t, first.Length, "with the first part")
		assert.Contains(t, first.Document(), fmt.Sprintf("length: %d\n", *first.Length))
		assert.NotContains(t, first.Document(), "content_size")
		part := second
		for part.Next != nil {
			part, err = workspace.ReadMarkdown(ctx, text("text/plain", long), *part.Next, 1000)
			require.NoError(t, err)
		}
		require.NotNil(t, part.Length)
		assert.Equal(t, part.Offset+uint64(len(part.Body)), *part.Length)
		assert.Equal(t, *first.Length, *part.Length, "the first part's length is where the last ends")
		assert.Contains(t, part.Document(), fmt.Sprintf("length: %d\n", *part.Length))

		// An offset beyond its end says so, once
		_, err = workspace.ReadMarkdown(ctx, text("text/plain", long), 100000, 100)
		require.ErrorIs(t, err, llm.ErrBadParameter)
		assert.Equal(t, 1, strings.Count(err.Error(), "bad parameter"), err.Error())
	})

	t.Run("LargeLength", func(t *testing.T) {
		// A resource larger than MaxLengthSize isn't converted whole for its
		// first part, which has no length
		large := strings.Repeat("Hello, world.\n", workspace.MaxLengthSize/14+1)
		part, err := workspace.ReadMarkdown(ctx, text("text/plain", large), 0, 100)
		require.NoError(t, err)
		require.NotNil(t, part.Next)
		assert.Nil(t, part.Length)
	})

	t.Run("NoText", func(t *testing.T) {
		// A page which its scripts render has its metadata, and a note
		page := `<!DOCTYPE html><html lang="en"><head><title>Profile</title></head><body><div id="root"></div><script>render()</script></body></html>`
		part, err := workspace.ReadMarkdown(ctx, text("text/html", page), 0, 0)
		require.NoError(t, err)
		assert.Empty(t, strings.TrimSpace(part.Body))
		assert.Equal(t, workspace.NoTextNote, part.Note)
		assert.Equal(t, "Profile", part.Meta["dc:title"])
	})

	t.Run("NotText", func(t *testing.T) {
		_, err := workspace.ReadMarkdown(ctx, text("audio/mpeg", "\xff\xfb"), 0, 0)
		assert.ErrorIs(t, err, llm.ErrNotImplemented)
		_, err = workspace.ReadMarkdown(ctx, nil, 0, 0)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}
