package schema

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Markdown is a part of a resource, read as Markdown: its body, from the
// byte of its Markdown at the offset, the offset of the next part, which is
// left out at its end, and, with the first part, its metadata, by namespace
// and key, such as dc:title
type Markdown struct {
	URI    string            `json:"uri" help:"URI of the resource"`
	Type   string            `json:"type,omitempty" help:"MIME type of the resource"`
	Body   string            `json:"body" help:"The part's Markdown"`
	Offset uint64            `json:"offset" help:"The byte of the Markdown the part starts at"`
	Next   *uint64           `json:"next,omitempty" help:"The offset of the next part, or none, at the end"`
	Size   *int64            `json:"size,omitempty" help:"The size of the resource, in bytes, when it's known, which, for text, is the size of its Markdown, whose offsets are its"`
	Meta   map[string]string `json:"meta,omitempty" help:"The resource's metadata, with the first part, by namespace and key, such as dc:title"`
	Note   string            `json:"note,omitempty" help:"Why the part has no text, such as a page which is rendered by its scripts"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Document returns the part as a Markdown document, whose front matter, in
// YAML, has its URI, type, offset, the offset of the next part, when there
// is one, its note, and its metadata, and whose body is its Markdown
func (m Markdown) Document() string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "uri: %s\n", quote(m.URI))
	if m.Type != "" {
		fmt.Fprintf(&b, "type: %s\n", quote(m.Type))
	}
	fmt.Fprintf(&b, "offset: %d\n", m.Offset)
	if m.Next != nil {
		fmt.Fprintf(&b, "next: %d\n", *m.Next)
	}
	if m.Size != nil {
		fmt.Fprintf(&b, "size: %d\n", *m.Size)
	}
	if m.Note != "" {
		fmt.Fprintf(&b, "note: %s\n", quote(m.Note))
	}
	if len(m.Meta) > 0 {
		b.WriteString("meta:\n")
		for _, key := range slices.Sorted(maps.Keys(m.Meta)) {
			fmt.Fprintf(&b, "  %s: %s\n", quote(key), quote(m.Meta[key]))
		}
	}
	b.WriteString("---\n")
	if body := strings.TrimSpace(m.Body); body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return b.String()
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// quote returns the text as a YAML string, which a JSON string is, so it's
// as it is, whatever it has, such as a colon
func quote(text string) string {
	data, err := json.Marshal(text)
	if err != nil {
		return `""`
	}
	return string(data)
}
