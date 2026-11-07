package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	metadata "github.com/mutablelogic/go-metadata"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"

	// The handlers which read files, and convert them to Markdown, which
	// don't need cgo, with images' dimensions, but not their EXIF, or
	// audio's, or video's, metadata, which need libexif, or FFmpeg
	_ "github.com/mutablelogic/go-metadata/application"
	_ "github.com/mutablelogic/go-metadata/image/raster"
	_ "github.com/mutablelogic/go-metadata/image/svg"
	_ "github.com/mutablelogic/go-metadata/message"
	_ "github.com/mutablelogic/go-metadata/text"
	_ "github.com/mutablelogic/go-metadata/video/subtitle"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Resources are the files of chats, and on their own, which the resource
// tool lists, and reads, such as a resource.Manager
type Resources interface {
	// List returns the resources, such as a chat's, the most recently used
	// first
	List(context.Context, schema.ResourceListRequest) (schema.ResourceList, error)

	// Open returns the resource, and its content, which is closed once it's
	// read
	Open(context.Context, schema.ResourceKey) (schema.Resource, io.ReadCloser, error)
}

// resourceTool is the tool which lists the chat's files, or reads one, as
// Markdown, a part at a time
type resourceTool struct {
	toolkit.Base
	resources Resources
}

// readInput is the input of the read tool
type readInput struct {
	URI    string `json:"uri,omitempty" help:"URI of the file, resource://<hash>, which an attachment, a tool's result, or the list of the chat's files says, or omitted, to list the chat's files" optional:""`
	Format string `json:"format,omitempty" enum:"markdown,raw" help:"How the file is read: as Markdown, which is the default, or raw, as the text it is, such as subtitles, or code, as they are, for a file which is text" optional:""`
	Offset uint64 `json:"offset,omitempty" help:"Byte of the file's Markdown, or of the file, when it's raw, to start reading at, to read the next part of a long file, which is next in the previous read" optional:""`
	Limit  uint64 `json:"limit,omitempty" help:"Most bytes to read, from 4 to 65536, which is 16384 when it's omitted" optional:""`
}

// namedReader is a reader of a file's content, with the file's name, rather
// than the name it's kept with, which is its hash, which go-metadata would
// otherwise take for its title
type namedReader struct {
	io.Reader
	name string
}

// Name returns the file's name
func (r namedReader) Name() string {
	return r.name
}

// readOutput is a part of a file, as Markdown, with its metadata, such as
// its title, with the first part, or what the file is, and its metadata,
// when it can't be read as Markdown, such as an image, or audio
type readOutput struct {
	URI      string            `json:"uri"`
	Name     string            `json:"name,omitempty"`
	Title    string            `json:"title,omitempty"`
	Type     string            `json:"type,omitempty"`
	Size     int64             `json:"size"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Offset   uint64            `json:"offset,omitempty"`
	Content  *string           `json:"content,omitempty"`
	Next     *uint64           `json:"next,omitempty"`
	Note     string            `json:"note,omitempty"`
}

// part is a part of a file's Markdown: its text, the byte of the Markdown it
// starts at, and the byte the next part starts at, or nil, at its end
type part struct {
	body   string
	offset uint64
	next   *uint64
	meta   map[string]string
}

var _ gollm.Tool = (*resourceTool)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// defaultReadLimit is how much of a file's Markdown is read, when it isn't
	// said, and maxReadLimit the most, so a read doesn't fill the context
	defaultReadLimit = 16 * 1024
	maxReadLimit     = 64 * 1024

	// maxListed is the most files which are listed
	maxListed = 100
)

// readNamespaces are the namespaces of a file's metadata which are read,
// with its Markdown: what it is, its Dublin Core, such as its title, author
// and date, and its format's, such as an image's width and height, apart
// from its artwork, which is expensive, and those which are looked up
// online, such as AcoustID's
var readNamespaces = []string{
	metadata.MarkdownNamespace, metadata.MetadataNamespace, "dc",
	"image", "exif", "iptc", "xmp", "tiff", "heif", "psd",
	"audio", "video", "quicktime", "subtitle",
	"pdf", "postscript", "msoffice", "iwork", "mobi", "rtf", "html", "xml", "csv",
	"ical", "vcard", "email", "mbox", "archive", "google", "android",
}

// errBeyondEnd is the error of a part which starts beyond the end of a
// file, which is a note, rather than an error, as a model which reads on
// can read too far
var errBeyondEnd = errors.New("the offset is beyond the end of the file")

const (
	// sniffBytes is how much of the start of a file is looked at, to know
	// whether it's text, before it's read, as Git does
	sniffBytes = 8000
)

// maxTextBytes is the most of a document of text, such as HTML, or XML,
// which go-metadata converts to Markdown, so the Markdown of a larger one
// may be of its start only
var maxTextBytes int64 = 16 << 20

// maxMetadataValue is the longest value of a file's metadata which is
// returned, in bytes, as some, such as XMP, can be long
const maxMetadataValue = 512

const (
	// notText is the note when a file can't be read as text, and beyondEnd
	// when the offset is beyond the end of it
	notText   = "The file can't be read as text, so only what it is is returned, and it can be given to a tool which reads it, by its URI."
	beyondEnd = "The offset is beyond the end of the file."

	// truncatedText is the note of the last part of the Markdown of a file
	// of text which is larger than go-metadata converts
	truncatedText = "The file is larger than 16 MiB, and its Markdown may be of its start only. Read it raw, a part at a time, for all of it."

	// unreadable is the note of a file which couldn't be read as Markdown,
	// such as a document which isn't what its type says, with why
	unreadable = "The file couldn't be read as Markdown: "

	// The notes, of what reads a file which can't be read as text, by its
	// type: an image, or a scan, whose text llm:ocr recognizes, and audio,
	// or video, whose speech llm:transcribe transcribes, and whose metadata,
	// such as its duration, isn't read here
	notTextImage = "The file is an image, or a document, which can't be read as text, so only what it is is returned. Recognize its text with the llm:ocr tool, by its URI."
	notTextAudio = "The file is audio, or video, which can't be read as text, and whose length, and other metadata, aren't read here, so only what it is is returned. Transcribe its speech with the llm:transcribe tool, by its URI."
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*resourceTool) Name() string {
	return "resource"
}

func (*resourceTool) Description() string {
	return "Lists the files of this chat, as a table, without a URI, or reads a file, by its URI, resource://<hash>, such as a document, a transcript, subtitles, a web page, an email, or data, as Markdown, rather than in the file's own format: a text file's text, a PDF's, or a Word, or OpenDocument, document's, a spreadsheet's tables, an email's messages, subtitles, or code and data in code blocks. A long file is read a part at a time: read the next part from next, when it's returned. With format raw, a file which is text, such as subtitles, or code, is read as it is, rather than as Markdown. The first part has the file's metadata, such as a document's title, author, date and pages, a book's publisher, or an image's width and height. A file which can't be read as text, such as an image, or audio, has no content, only what it is, its metadata, when it has any, and a note of the tool which reads it."
}

func (*resourceTool) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[readInput]()
}

// Run lists the files of the chat, as Markdown, without a URI, or reads the
// part of the file, from the offset, up to the limit, as Markdown, or says
// what the file is, when it can't be read as Markdown
func (t *resourceTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req readInput
	if len(input) > 0 {
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, gollm.ErrBadParameter.With(err.Error())
		}
	}
	session := toolkit.SessionFromContext(ctx).ID()
	if strings.TrimSpace(req.URI) == "" {
		return t.list(ctx, session)
	}
	limit := req.Limit
	if limit == 0 {
		limit = defaultReadLimit
	} else if limit < utf8.UTFMax {
		// A part has a whole character, at least, so a long file is read on
		return nil, gollm.ErrBadParameter.Withf("limit must be at least %d", utf8.UTFMax)
	}
	limit = min(limit, maxReadLimit)

	// The file, in the chat, or on its own, which is read as it's opened
	file, content, err := t.open(ctx, strings.TrimSpace(req.URI), session)
	if err != nil {
		return nil, err
	}
	defer content.Close()
	output := readOutput{URI: file.URI(), Name: file.Name, Title: file.Description, Type: file.Type, Size: file.Size}
	toolkit.Logf(ctx, "Reading %s", describeFile(file))

	// Whether it's text is known from its start, once, so each part of it
	// is, or isn't, rather than only those which have a NUL
	scan := &nulScanner{Reader: content}
	head := bufio.NewReaderSize(scan, sniffBytes)
	start, _ := head.Peek(sniffBytes)
	binary := textType(file.Type) && !providerschema.IsText(start, file.Type)

	// hasNUL reads the rest of a file of text, which wasn't read, and
	// returns whether it has a NUL, anywhere, so it isn't text, as a part
	// which hasn't one would otherwise be, though another has
	hasNUL := func() bool {
		if !textType(file.Type) {
			return false
		}
		_, _ = io.Copy(io.Discard, head)
		return scan.found
	}

	// Its part, as it is, when it's raw, which is text
	if strings.EqualFold(strings.TrimSpace(req.Format), "raw") {
		if binary || !providerschema.IsText(start, file.Type) {
			output.Note = notTextFor(file.Type)
			return output, nil
		}
		text, offset, next, err := raw(head, file.Type, req.Offset, limit)
		switch {
		case errors.Is(err, errBeyondEnd):
			output.Note = beyondEnd
		case errors.Is(err, gollm.ErrNotImplemented):
			output.Note = notTextFor(file.Type)
		case err != nil:
			return nil, err
		case hasNUL():
			output.Note = notTextFor(file.Type)
		default:
			output.Offset, output.Content, output.Next = offset, &text, next
		}
		return output, nil
	} else if format := strings.TrimSpace(req.Format); format != "" && !strings.EqualFold(format, "markdown") {
		return nil, gollm.ErrBadParameter.Withf("unknown format %q, which isn't markdown or raw", format)
	} else if binary {
		output.Note = notTextFor(file.Type)
		return output, nil
	}

	// Its part, as Markdown, and, with the first, its metadata, whose title
	// is the file's own, when it has one
	p, err := markdown(ctx, namedReader{Reader: head, name: file.Name}, file.Type, req.Offset, limit)
	if p != nil && req.Offset == 0 {
		output.Metadata = p.meta
		if title := firstLine(file.Description); title != "" && output.Metadata != nil {
			output.Metadata["dc:title"] = title
		}
	}
	switch {
	case errors.Is(err, errBeyondEnd):
		output.Note = beyondEnd
	case errors.Is(err, gollm.ErrNotImplemented):
		output.Note = notTextFor(file.Type)
	case errors.Is(err, gollm.ErrBadParameter):
		// It isn't what its type says, such as a document which is cut short
		reason := err.Error()
		for strings.HasPrefix(reason, "bad parameter: ") {
			reason = strings.TrimPrefix(reason, "bad parameter: ")
		}
		output.Note = unreadable + reason
	case err != nil:
		return nil, err
	case hasNUL():
		output.Note = notTextFor(file.Type)
	default:
		output.Offset, output.Content, output.Next = p.offset, &p.body, p.next
		if p.next == nil && file.Size > maxTextBytes && textType(file.Type) {
			output.Note = truncatedText
		}
	}
	return output, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// list returns the files of the chat, the most recently used first, as a
// Markdown table, with their names, titles, types, sizes and URIs, or that
// there are none
func (t *resourceTool) list(ctx context.Context, session uuid.UUID) (string, error) {
	if session == uuid.Nil() {
		return "", gollm.ErrBadParameter.With("files are a chat's, and this isn't one, so give the URI of a file to read")
	}
	toolkit.Logf(ctx, "Looking at the chat's files")
	list, err := t.resources.List(ctx, schema.ResourceListRequest{Session: session.String(), Limit: types.Ptr(uint(maxListed))})
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
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", cell(file.Name), cell(firstLine(file.Description)), cell(file.Type), fileSize(file.Size), file.URI())
	}
	return b.String(), nil
}

// open returns the resource with the URI, in the session, or on its own,
// when it isn't the session's, or there's no session, and its content
func (t *resourceTool) open(ctx context.Context, uri string, session uuid.UUID) (schema.Resource, io.ReadCloser, error) {
	hash, err := schema.ParseURI(uri)
	if err != nil {
		return schema.Resource{}, nil, err
	}
	if session != uuid.Nil() {
		file, content, err := t.resources.Open(ctx, schema.ResourceKey{Hash: hash, Session: &session})
		if !errors.Is(err, gollm.ErrNotFound) {
			return file, content, err
		}
	}
	return t.resources.Open(ctx, schema.ResourceKey{Hash: hash})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// markdown reads the content, of the type, as Markdown, from the byte of
// its Markdown at the offset, up to the limit, with its metadata, other
// than its Markdown, by namespace and key, such as dc:title. Returns the
// part, with its metadata, and ErrNotImplemented when it has no Markdown,
// such as audio, or it isn't text, as it has a NUL, though its type says it
// is, or the part, without its metadata, and ErrNotImplemented, when its
// type isn't known, and ErrBadParameter when the offset is beyond its end.
func markdown(ctx context.Context, r io.Reader, contentType string, offset, limit uint64) (*part, error) {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, gollm.ErrNotImplemented.Withf("no Markdown, as the type %q isn't known", contentType)
	}
	_, meta, err := metadata.Extract(ctx, r,
		metadata.WithContentType(mediatype),
		metadata.WithNamespace(readNamespaces...),
		metadata.WithRange(fmt.Sprintf("%d:%d", offset, offset+limit)),
	)
	switch {
	case errors.Is(err, metadata.ErrNotImplemented):
		return nil, gollm.ErrNotImplemented.Withf("no Markdown of type %q", mediatype)
	case errors.Is(err, metadata.ErrBadParameter) && offset > 0 && strings.Contains(err.Error(), "beyond the end"):
		return nil, errBeyondEnd
	case errors.Is(err, metadata.ErrBadParameter):
		return nil, gollm.ErrBadParameter.With(err.Error())
	case err != nil:
		return nil, err
	}

	// The part's text, and where it starts, and the next starts, and its
	// metadata, apart from the hints of what's read next, which are a
	// handler's, rather than the file's
	p := &part{offset: offset}
	var hasBody bool
	for _, entry := range meta {
		switch {
		case entry.NS() != metadata.MarkdownNamespace:
			if entry.NS() == metadata.MetadataNamespace && entry.Key() == metadata.HintKey {
				continue
			}
			if value := strings.TrimSpace(entry.Value()); value != "" {
				if p.meta == nil {
					p.meta = make(map[string]string)
				}
				p.meta[entry.NS()+":"+entry.Key()] = truncateText(value, maxMetadataValue)
			}
		case entry.Key() == "body":
			p.body, hasBody = entry.Value(), true
		case entry.Key() == "offset":
			if value, ok := entry.Any().(uint64); ok {
				p.offset = value
			}
		case entry.Key() == "next":
			if value, ok := entry.Any().(uint64); ok {
				p.next = &value
			}
		}
	}
	if !hasBody {
		return p, gollm.ErrNotImplemented.Withf("no Markdown of type %q", mediatype)
	} else if strings.ContainsRune(p.body, 0) {
		// A file which has a NUL isn't text, though its type says it is
		return &part{meta: p.meta}, gollm.ErrNotImplemented.Withf("no Markdown, as the file of type %q isn't text", mediatype)
	}
	return p, nil
}

// nulScanner is a reader which notes whether what's read has a NUL
type nulScanner struct {
	io.Reader
	found bool
}

func (s *nulScanner) Read(p []byte) (int, error) {
	n, err := s.Reader.Read(p)
	if !s.found && bytes.IndexByte(p[:n], 0) >= 0 {
		s.found = true
	}
	return n, err
}

// textType returns whether the type is of text, such as text/plain, HTML,
// JSON, XML or subtitles, rather than of a document which is binary, such
// as a Word document, which can have NULs, and isn't text, though it can be
// read as Markdown
func textType(contentType string) bool {
	mediatype, _, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasPrefix(mediatype, "text/"), strings.HasSuffix(mediatype, "+json"), strings.HasSuffix(mediatype, "+xml"):
		return true
	}
	switch mediatype {
	case "application/json", "application/xml", "application/x-subrip", "application/javascript", "application/x-yaml", "application/yaml", "application/toml", "application/x-sh", "application/sql", "application/x-ndjson":
		return true
	}
	return false
}

// notTextFor returns the note of a file, of the type, which can't be read
// as text, which says which tool reads it
func notTextFor(contentType string) string {
	mediatype, _, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasPrefix(mediatype, "image/"), mediatype == "application/pdf":
		return notTextImage
	case strings.HasPrefix(mediatype, "audio/"), strings.HasPrefix(mediatype, "video/"):
		return notTextAudio
	}
	return notText
}

// raw reads the content, which is text, as it is, from the byte at the
// offset, up to the limit, ending at a whole character, and returns it,
// where it starts, and where the next part starts, or nil, at its end.
// Returns ErrNotImplemented when it isn't text, such as an image, or has a
// NUL, or isn't UTF-8, and ErrBadParameter when the offset is beyond its
// end.
func raw(r io.Reader, contentType string, offset, limit uint64) (string, uint64, *uint64, error) {
	if !providerschema.IsText(nil, contentType) {
		return "", 0, nil, gollm.ErrNotImplemented.Withf("%q isn't text", contentType)
	}
	if skipped, err := io.CopyN(io.Discard, r, int64(offset)); err != nil && !errors.Is(err, io.EOF) {
		return "", 0, nil, err
	} else if uint64(skipped) < offset {
		return "", 0, nil, errBeyondEnd
	}

	// The part, and a byte more, to know whether there's more of it, which
	// starts at a whole character, and ends at one
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)+utf8.UTFMax))
	if err != nil {
		return "", 0, nil, err
	} else if len(data) == 0 && offset > 0 {
		return "", 0, nil, errBeyondEnd
	}
	start := 0
	for start < len(data) && start < utf8.UTFMax && !utf8.RuneStart(data[start]) {
		start++
	}
	end := min(len(data), start+int(limit))
	for end > start && end < len(data) && !utf8.RuneStart(data[end]) {
		end--
	}
	part := data[start:end]
	if !providerschema.IsText(part, contentType) {
		return "", 0, nil, gollm.ErrNotImplemented.Withf("%q isn't text", contentType)
	}
	var next *uint64
	if end < len(data) {
		next = types.Ptr(offset + uint64(end))
	}
	return string(part), offset + uint64(start), next, nil
}

// describeFile returns what a file is, for people: its name, or its type
func describeFile(file schema.Resource) string {
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

// fileSize returns the size, in bytes, for people, such as 1.2 MB
func fileSize(n int64) string {
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

// Timeout returns the most time the tool has, which is longer than a tool
// has by default, for reading a long document, such as a PDF
func (*resourceTool) Timeout() time.Duration {
	return resourceTimeout
}
