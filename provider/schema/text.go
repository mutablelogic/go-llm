package schema

import (
	"bytes"
	"fmt"
	"mime"
	"strings"
	"unicode/utf8"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// IsText returns true when the data is text, which is UTF-8, without NUL
// bytes, and isn't of a type, such as a PDF or an image, which is sent to a
// model as it is, even when it's UTF-8
func IsText(data []byte, contentType string) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasPrefix(mediaType, "image/"), strings.HasPrefix(mediaType, "audio/"), strings.HasPrefix(mediaType, "video/"), mediaType == "application/pdf":
		return false
	case len(data) == 0:
		return true
	}
	return utf8.Valid(data) && !bytes.ContainsRune(data, 0)
}

// TextBlock returns the content block of a file of text, such as code or
// subtitles, which is sent to a model as text, rather than as an attachment,
// which most models don't accept for text: its name, and then its text, in
// a fenced code block
func TextBlock(name string, data []byte) ContentBlock {
	text := strings.TrimRight(string(data), "\n")
	fence := CodeFence(text)
	return ContentBlock{Text: types.Ptr(fmt.Sprintf("%s:\n\n%s\n%s\n%s", name, fence, text, fence))}
}

// CodeFence returns the fence of a code block of the text, of backticks,
// which is longer than any run of backticks in the text, and at least
// three, so a fenced code block in the text, such as of a Markdown file,
// doesn't close it
func CodeFence(text string) string {
	longest, run := 0, 0
	for _, r := range text {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

// Unreadable returns the text which is sent to a model in place of an
// attachment it can't read, such as audio, for a model which doesn't hear
// it, or an image in a format its provider doesn't take, which says its
// URI, so the model can give it to a tool which reads it
func Unreadable(uri, contentType string) string {
	if uri == "" {
		uri = "an attachment"
	}
	return "(The attachment " + uri + " is " + contentType + ", which this model can't read, so it isn't sent. Its URI is for tools, which can read it.)"
}
