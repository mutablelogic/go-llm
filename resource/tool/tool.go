// Package tool is the resource namespace of tools, whose read tool lists
// the files of the chat it's called in, or reads a file, by its URI, as
// Markdown, a part at a time
package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"
	"unicode/utf8"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	resource "github.com/mutablelogic/go-llm/resource"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	metadata "github.com/mutablelogic/go-metadata"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"

	// The handlers which convert files to Markdown, which don't need cgo
	_ "github.com/mutablelogic/go-metadata/application"
	_ "github.com/mutablelogic/go-metadata/image/svg"
	_ "github.com/mutablelogic/go-metadata/message"
	_ "github.com/mutablelogic/go-metadata/text"
	_ "github.com/mutablelogic/go-metadata/video/subtitle"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Namespace is the resource namespace, whose read tool reads the files of
// the resource manager
type Namespace struct {
	manager *resource.Manager
}

// read is the tool which lists the chat's files, or reads one, as Markdown,
// a part at a time
type read struct {
	toolkit.Base
	manager *resource.Manager
}

// readInput is the input of the read tool
type readInput struct {
	URI    string `json:"uri,omitempty" help:"URI of the file, resource://<hash>, which an attachment, a tool's result, or the list of the chat's files says, or omitted, to list the chat's files" optional:""`
	Offset uint64 `json:"offset,omitempty" help:"Byte of the file's Markdown to start reading at, to read the next part of a long file, which is next in the previous read" optional:""`
	Limit  uint64 `json:"limit,omitempty" help:"Most bytes to read, from 4 to 65536, which is 16384 when it's omitted" optional:""`
}

// readOutput is a part of a file, as Markdown, or what the file is, when it
// can't be read as Markdown, such as an image, or audio
type readOutput struct {
	URI     string  `json:"uri"`
	Name    string  `json:"name,omitempty"`
	Title   string  `json:"title,omitempty"`
	Type    string  `json:"type,omitempty"`
	Size    int64   `json:"size"`
	Offset  uint64  `json:"offset,omitempty"`
	Content *string `json:"content,omitempty"`
	Next    *uint64 `json:"next,omitempty"`
	Note    string  `json:"note,omitempty"`
}

// part is a part of a file's Markdown: its text, the byte of the Markdown it
// starts at, and the byte the next part starts at, or nil, at its end
type part struct {
	body   string
	offset uint64
	next   *uint64
}

var _ toolkit.Namespace = (*Namespace)(nil)
var _ llm.Tool = (*read)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace
const Name = "resource"

const (
	// defaultLimit is how much of a file's Markdown is read, when it isn't
	// said, and maxLimit the most, so a read doesn't fill the context
	defaultLimit = 16 * 1024
	maxLimit     = 64 * 1024

	// maxList is the most files which are listed
	maxList = 100
)

const (
	// notText is the note when a file can't be read as Markdown, and
	// beyondEnd when the offset is beyond the end of it
	notText   = "the file can't be read as text, such as an image, or audio, so only what it is is returned, and it can be given to a tool which reads it, by its URI"
	beyondEnd = "the offset is beyond the end of the file"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the namespace, whose tool reads the files the manager keeps
func New(manager *resource.Manager) (*Namespace, error) {
	if manager == nil {
		return nil, llm.ErrBadParameter.With("missing resource manager")
	}
	return &Namespace{manager: manager}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - NAMESPACE

func (*Namespace) Name() string {
	return Name
}

func (*Namespace) Description() string {
	return "The files of a chat, such as documents, transcripts and subtitles, which are read as Markdown"
}

func (*Namespace) SystemPrompt() string {
	return ""
}

func (n *Namespace) Tools() []llm.Tool {
	return []llm.Tool{&read{manager: n.manager}}
}

// Notify does nothing, as the namespace's tools don't change
func (*Namespace) Notify(func(eventschema.EventAction, string)) {}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - TOOL

func (*read) Name() string {
	return "read"
}

func (*read) Description() string {
	return "Lists the files of this chat, as a table, without a URI, or reads a file, by its URI, resource://<hash>, such as a document, a transcript, subtitles, a web page, an email, or data, as Markdown, rather than in the file's own format: a text file's text, a PDF's, or a Word, or OpenDocument, document's, a spreadsheet's tables, an email's messages, subtitles, or code and data in code blocks. A long file is read a part at a time: read the next part from next, when it's returned. A file which can't be read as text, such as an image, or audio, has no content, only what it is, and a note."
}

func (*read) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[readInput]()
}

// Run lists the files of the chat, as Markdown, without a URI, or reads the
// part of the file, from the offset, up to the limit, as Markdown, or says
// what the file is, when it can't be read as Markdown
func (t *read) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req readInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, llm.ErrBadParameter.With(err.Error())
		}
	}
	session := toolkit.SessionFromContext(ctx).ID()
	if strings.TrimSpace(req.URI) == "" {
		return t.list(ctx, session)
	}
	limit := req.Limit
	if limit == 0 {
		limit = defaultLimit
	} else if limit < utf8.UTFMax {
		// A part has a whole character, at least, so a long file is read on
		return nil, llm.ErrBadParameter.Withf("limit must be at least %d", utf8.UTFMax)
	}
	limit = min(limit, maxLimit)

	// The file, in the chat, or on its own, which is read as it's opened
	file, content, err := t.open(ctx, strings.TrimSpace(req.URI), session)
	if err != nil {
		return nil, err
	}
	defer content.Close()
	output := readOutput{URI: file.URI(), Name: file.Name, Title: file.Description, Type: file.Type, Size: file.Size}
	toolkit.Logf(ctx, "Reading %s", describe(file))

	// Its part, as Markdown
	p, err := markdown(ctx, content, file.Type, req.Offset, limit)
	switch {
	case errors.Is(err, llm.ErrNotImplemented):
		output.Note = notText
	case errors.Is(err, llm.ErrBadParameter) && req.Offset > 0:
		output.Note = beyondEnd
	case err != nil:
		return nil, err
	default:
		output.Offset, output.Content, output.Next = p.offset, &p.body, p.next
	}
	return output, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// list returns the files of the chat, the most recently used first, as a
// Markdown table, with their names, titles, types, sizes and URIs, or that
// there are none
func (t *read) list(ctx context.Context, session uuid.UUID) (string, error) {
	if session == uuid.Nil() {
		return "", llm.ErrBadParameter.With("files are a chat's, and this isn't one, so give the URI of a file to read")
	}
	toolkit.Logf(ctx, "Looking at the chat's files")
	list, err := t.manager.List(ctx, schema.ResourceListRequest{Session: session.String(), Limit: types.Ptr(uint(maxList))})
	if err != nil {
		return "", err
	} else if len(list.Body) == 0 {
		return "This chat has no files.", nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "This chat has %d %s", list.Count, plural(list.Count, "file", "files"))
	if list.Count > uint(len(list.Body)) {
		fmt.Fprintf(&b, ", of which the %d most recently used are", len(list.Body))
	}
	b.WriteString(":\n\n| Name | Title | Type | Size | URI |\n| --- | --- | --- | --- | --- |\n")
	for _, file := range list.Body {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", cell(file.Name), cell(firstLine(file.Description)), cell(file.Type), size(file.Size), file.URI())
	}
	return b.String(), nil
}

// open returns the resource with the URI, in the session, or on its own,
// when it isn't the session's, or there's no session, and its content
func (t *read) open(ctx context.Context, uri string, session uuid.UUID) (schema.Resource, io.ReadCloser, error) {
	hash, err := schema.ParseURI(uri)
	if err != nil {
		return schema.Resource{}, nil, err
	}
	if session != uuid.Nil() {
		file, content, err := t.manager.Open(ctx, schema.ResourceKey{Hash: hash, Session: &session})
		if !errors.Is(err, llm.ErrNotFound) {
			return file, content, err
		}
	}
	return t.manager.Open(ctx, schema.ResourceKey{Hash: hash})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// markdown reads the content, of the type, as Markdown, from the byte of
// its Markdown at the offset, up to the limit. Returns ErrNotImplemented
// when it has no Markdown, such as audio, or its type isn't known, or it
// isn't text, as it has a NUL, though its type says it is, and
// ErrBadParameter when the offset is beyond its end.
func markdown(ctx context.Context, r io.Reader, contentType string, offset, limit uint64) (*part, error) {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, llm.ErrNotImplemented.Withf("no Markdown, as the type %q isn't known", contentType)
	}
	_, meta, err := metadata.Extract(ctx, r,
		metadata.WithContentType(mediatype),
		metadata.WithNamespace(metadata.MarkdownNamespace),
		metadata.WithRange(fmt.Sprintf("%d:%d", offset, offset+limit)),
	)
	switch {
	case errors.Is(err, metadata.ErrNotImplemented):
		return nil, llm.ErrNotImplemented.Withf("no Markdown of type %q", mediatype)
	case errors.Is(err, metadata.ErrBadParameter) && offset > 0:
		return nil, llm.ErrBadParameter.Withf("offset %d is beyond the end of the Markdown", offset)
	case err != nil:
		return nil, err
	}

	// The part's text, and where it starts, and the next starts
	var result *part
	p := part{offset: offset}
	for _, entry := range meta {
		if entry.NS() != metadata.MarkdownNamespace {
			continue
		}
		switch entry.Key() {
		case "body":
			p.body, result = entry.Value(), &p
		case "offset":
			if value, ok := entry.Any().(uint64); ok {
				p.offset = value
			}
		case "next":
			if value, ok := entry.Any().(uint64); ok {
				p.next = &value
			}
		}
	}
	if result == nil {
		return nil, llm.ErrNotImplemented.Withf("no Markdown of type %q", mediatype)
	} else if strings.ContainsRune(result.body, 0) {
		// A file which has a NUL isn't text, though its type says it is
		return nil, llm.ErrNotImplemented.Withf("no Markdown, as the file of type %q isn't text", mediatype)
	}
	return result, nil
}

// describe returns what a file is, for people: its name, or its type
func describe(file schema.Resource) string {
	if name := strings.TrimSpace(file.Name); name != "" {
		return name
	} else if t := strings.TrimSpace(file.Type); t != "" {
		return "a file of type " + t
	}
	return "a file"
}

// cell returns the value, in a Markdown table's cell, without the pipes and
// line breaks which would end it
func cell(value string) string {
	value = strings.NewReplacer("|", "\\|", "\r", " ", "\n", " ").Replace(strings.TrimSpace(value))
	if value == "" {
		return "-"
	}
	return value
}

// firstLine returns the first line of the text
func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

// size returns the size, in bytes, for people, such as 1.2 MB
func size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

// plural returns the singular, for one, or the plural
func plural(n uint, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
