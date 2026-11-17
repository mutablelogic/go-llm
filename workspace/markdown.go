package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"
	"unicode/utf8"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	metadata "github.com/mutablelogic/go-metadata"

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
// GLOBALS

const (
	// DefaultMarkdownLimit is how much of a resource's Markdown is read,
	// when it isn't said, and MaxMarkdownLimit the most, so a part doesn't
	// fill a model's context
	DefaultMarkdownLimit = 16 * 1024
	MaxMarkdownLimit     = 64 * 1024

	// NoTextNote is the note of a resource which has no text, though it's
	// read as Markdown, such as a page which its scripts render, as an MCP
	// app's is, of which only its metadata is returned
	NoTextNote = "The resource has no text, such as a page which is rendered by its scripts, so only its metadata is returned. Read it raw for its content."

	// maxMetaValue is the longest value of a resource's metadata which is
	// returned with its Markdown
	maxMetaValue = 512
)

// markdownNamespaces are the namespaces of a resource's metadata which are
// read, with its Markdown: what it is, its Dublin Core, such as its title,
// author and date, and its format's, apart from its artwork, which is
// expensive, and those which are looked up online
var markdownNamespaces = []string{
	metadata.MarkdownNamespace, metadata.MetadataNamespace, "dc",
	"image", "exif", "iptc", "xmp", "tiff", "heif", "psd",
	"audio", "video", "quicktime", "subtitle",
	"pdf", "postscript", "msoffice", "iwork", "mobi", "rtf", "html", "xml", "csv",
	"ical", "vcard", "email", "mbox", "archive", "google", "android",
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// namedReader is a resource's content, with its name, whose extension
// go-metadata uses to know what it is
type namedReader struct {
	io.Reader
	name string
}

func (r namedReader) Name() string { return r.name }

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// ReadMarkdown reads the resource as Markdown, with go-metadata, which reads
// text, HTML, PDFs, Office documents, email and more: a part of it, from
// the byte of its Markdown at the offset, of at most the limit, which is
// DefaultMarkdownLimit, when it's zero, and at most MaxMarkdownLimit, and,
// with the first part, its metadata. Returns ErrNotImplemented when it has
// no Markdown, such as audio, or it isn't text, and ErrBadParameter when
// the offset is beyond its end, or it isn't what its type says.
func ReadMarkdown(ctx context.Context, resource llm.Resource, offset, limit uint64) (*schema.Markdown, error) {
	if resource == nil {
		return nil, llm.ErrBadParameter.With("missing resource")
	}
	if limit == 0 {
		limit = DefaultMarkdownLimit
	}
	limit = min(limit, MaxMarkdownLimit)
	mediatype, _, err := mime.ParseMediaType(resource.Type())
	if err != nil {
		return nil, llm.ErrNotImplemented.Withf("no Markdown, as the type %q isn't known", resource.Type())
	}
	read, err := resource.Read(ctx)
	if err != nil {
		return nil, err
	}

	// Its Markdown, and metadata
	var meta []metadata.Metadata
	if err := read(ctx, func(r io.Reader) error {
		var err error
		opts := []metadata.Option{
			metadata.WithContentType(mediatype),
			metadata.WithNamespace(markdownNamespaces...),
			metadata.WithRange(fmt.Sprintf("%d:%d", offset, offset+limit)),
		}
		// The first part has the length of all of the Markdown, so how much
		// there is is known, which converts all of it, once
		if offset == 0 {
			opts = append(opts, metadata.WithLength())
		}
		_, meta, err = metadata.Extract(ctx, namedReader{Reader: r, name: resource.Name()}, opts...)
		return err
	}); err != nil {
		switch {
		case errors.Is(err, metadata.ErrNotImplemented):
			return nil, llm.ErrNotImplemented.Withf("no Markdown of type %q", mediatype)
		case errors.Is(err, metadata.ErrBadParameter):
			// Its reason, without go-metadata's kind, which is the same
			return nil, llm.ErrBadParameter.With(strings.TrimPrefix(err.Error(), metadata.ErrBadParameter.Error()+": "))
		default:
			return nil, err
		}
	}

	// The part's body, where it starts, and the next starts, and the
	// resource's metadata, with the first part, apart from what's read next,
	// which is go-metadata's
	result := &schema.Markdown{URI: resource.URI(), Type: resource.Type(), Offset: offset}
	var hasBody bool
	for _, entry := range meta {
		switch {
		case entry.NS() != metadata.MarkdownNamespace:
			if offset > 0 || (entry.NS() == metadata.MetadataNamespace && entry.Key() == metadata.HintKey) {
				continue
			}
			if value := strings.TrimSpace(entry.Value()); value != "" {
				if result.Meta == nil {
					result.Meta = make(map[string]string)
				}
				result.Meta[entry.NS()+":"+entry.Key()] = truncate(value, maxMetaValue)
			}
		case entry.Key() == "body":
			result.Body, hasBody = entry.Value(), true
		case entry.Key() == "offset":
			if value, ok := entry.Any().(uint64); ok {
				result.Offset = value
			}
		case entry.Key() == "next":
			if value, ok := entry.Any().(uint64); ok {
				result.Next = &value
			}
		case entry.Key() == "length":
			if value, ok := entry.Any().(uint64); ok {
				result.Length = &value
			}
		}
	}
	if hasBody && result.Next == nil && result.Length == nil {
		// The last part, whose end is the Markdown's
		length := result.Offset + uint64(len(result.Body))
		result.Length = &length
	}
	if !hasBody {
		return nil, llm.ErrNotImplemented.Withf("no Markdown of type %q", mediatype)
	} else if strings.ContainsRune(result.Body, 0) {
		return nil, llm.ErrNotImplemented.Withf("no Markdown, as the resource of type %q isn't text", mediatype)
	} else if strings.TrimSpace(result.Body) == "" && offset == 0 {
		result.Note = NoTextNote
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// truncate returns the start of the text, of at most the length, in bytes,
// which doesn't end in the middle of a character
func truncate(text string, length int) string {
	if len(text) <= length {
		return text
	}
	cut := length
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}
