package telegram

import (
	"fmt"
	"strings"

	// Packages
	gte "github.com/igor-pavlenko/goldmark-telegram/extension"
	gteast "github.com/igor-pavlenko/goldmark-telegram/extension/ast"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	tele "gopkg.in/telebot.v4"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// markdownToEntities converts standard Markdown into plain text plus
// Telegram MessageEntity objects by walking the goldmark AST directly.
// This handles block-level elements (paragraphs, lists, headings, code
// blocks, blockquotes) that goldmark-telegram's renderer ignores.
func markdownToEntities(markdown string) (string, tele.Entities) {
	source := []byte(markdown)

	// Parse using goldmark with GTE extensions (strikethrough, underline)
	p := goldmark.New(goldmark.WithExtensions(gte.GTE))
	reader := text.NewReader(source)
	doc := p.Parser().Parse(reader)

	// Walk the AST and build text + entities
	b := &entityBuilder{}
	b.walkNode(doc, source)

	result := strings.TrimRight(b.text.String(), "\n")
	if len(b.entities) == 0 {
		return result, nil
	}
	return result, b.entities
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE TYPES

type entityBuilder struct {
	text     strings.Builder
	entities tele.Entities
	utf16Off int // current offset in UTF-16 code units
	listItem int // 1-based ordered list counter, 0 for unordered
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// writeString appends s to the text buffer and advances the UTF-16 offset.
func (b *entityBuilder) writeString(s string) {
	b.text.WriteString(s)
	b.utf16Off += utf16Len(s)
}

// ensureNewline writes a newline only if the buffer doesn't already end with one.
func (b *entityBuilder) ensureNewline() {
	if b.text.Len() > 0 {
		s := b.text.String()
		if s[len(s)-1] != '\n' {
			b.writeString("\n")
		}
	}
}

// blockSeparator inserts a blank line before a new block if there is already content.
func (b *entityBuilder) blockSeparator() {
	if b.text.Len() == 0 {
		return
	}
	s := b.text.String()
	n := len(s)
	switch {
	case n >= 2 && s[n-2] == '\n' && s[n-1] == '\n':
		// already double newline
	case n >= 1 && s[n-1] == '\n':
		b.writeString("\n")
	default:
		b.writeString("\n\n")
	}
}

func (b *entityBuilder) walkNode(node ast.Node, source []byte) {
	switch n := node.(type) {
	case *ast.Document:
		b.walkChildren(n, source)

	case *ast.Paragraph:
		b.blockSeparator()
		b.walkChildren(n, source)

	case *ast.TextBlock:
		// Tight list item content — no blank line before
		b.walkChildren(n, source)

	case *ast.Heading:
		b.blockSeparator()
		start := b.utf16Off
		b.walkChildren(n, source)
		if length := b.utf16Off - start; length > 0 {
			b.entities = append(b.entities, tele.MessageEntity{
				Type:   tele.EntityBold,
				Offset: start,
				Length: length,
			})
		}

	case *ast.List:
		b.ensureNewline()
		savedItem := b.listItem
		if n.IsOrdered() {
			b.listItem = n.Start
			if b.listItem == 0 {
				b.listItem = 1
			}
		} else {
			b.listItem = 0
		}
		b.walkChildren(n, source)
		b.listItem = savedItem

	case *ast.ListItem:
		b.ensureNewline()
		if b.listItem > 0 {
			b.writeString(fmt.Sprintf("%d. ", b.listItem))
			b.listItem++
		} else {
			b.writeString("• ")
		}
		b.walkChildren(n, source)

	case *ast.Blockquote:
		b.blockSeparator()
		start := b.utf16Off
		b.walkChildren(n, source)
		if length := b.utf16Off - start; length > 0 {
			b.entities = append(b.entities, tele.MessageEntity{
				Type:   tele.EntityBlockquote,
				Offset: start,
				Length: length,
			})
		}

	case *ast.FencedCodeBlock:
		b.blockSeparator()
		start := b.utf16Off
		lines := n.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			b.writeString(string(seg.Value(source)))
		}
		// Remove trailing newline inside code block text for cleaner display
		txt := b.text.String()
		if len(txt) > 0 && txt[len(txt)-1] == '\n' {
			b.text.Reset()
			b.text.WriteString(txt[:len(txt)-1])
			b.utf16Off--
		}
		if length := b.utf16Off - start; length > 0 {
			b.entities = append(b.entities, tele.MessageEntity{
				Type:     tele.EntityCodeBlock,
				Offset:   start,
				Length:   length,
				Language: string(n.Language(source)),
			})
		}
		return // children are the code lines, already handled

	case *ast.CodeBlock:
		b.blockSeparator()
		start := b.utf16Off
		lines := n.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			b.writeString(string(seg.Value(source)))
		}
		txt := b.text.String()
		if len(txt) > 0 && txt[len(txt)-1] == '\n' {
			b.text.Reset()
			b.text.WriteString(txt[:len(txt)-1])
			b.utf16Off--
		}
		if length := b.utf16Off - start; length > 0 {
			b.entities = append(b.entities, tele.MessageEntity{
				Type:   tele.EntityCodeBlock,
				Offset: start,
				Length: length,
			})
		}
		return

	case *ast.ThematicBreak:
		b.blockSeparator()
		b.writeString("———")
		return

	case *ast.Emphasis:
		start := b.utf16Off
		b.walkChildren(n, source)
		entityType := tele.EntityItalic
		if n.Level == 2 {
			entityType = tele.EntityBold
		}
		if length := b.utf16Off - start; length > 0 {
			b.entities = append(b.entities, tele.MessageEntity{
				Type:   entityType,
				Offset: start,
				Length: length,
			})
		}

	case *ast.CodeSpan:
		start := b.utf16Off
		// Render code span children (raw text segments)
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				seg := t.Segment
				b.writeString(string(seg.Value(source)))
			}
		}
		if length := b.utf16Off - start; length > 0 {
			b.entities = append(b.entities, tele.MessageEntity{
				Type:   tele.EntityCode,
				Offset: start,
				Length: length,
			})
		}
		return // children already handled

	case *ast.Link:
		start := b.utf16Off
		b.walkChildren(n, source)
		if length := b.utf16Off - start; length > 0 {
			b.entities = append(b.entities, tele.MessageEntity{
				Type:   tele.EntityTextLink,
				Offset: start,
				Length: length,
				URL:    string(n.Destination),
			})
		}

	case *ast.AutoLink:
		url := string(n.URL(source))
		b.writeString(url)
		return

	case *ast.Image:
		// Render alt text as plain text, link the URL
		start := b.utf16Off
		b.walkChildren(n, source)
		if length := b.utf16Off - start; length > 0 {
			b.entities = append(b.entities, tele.MessageEntity{
				Type:   tele.EntityTextLink,
				Offset: start,
				Length: length,
				URL:    string(n.Destination),
			})
		}

	case *ast.Text:
		seg := n.Segment
		b.writeString(string(seg.Value(source)))
		if n.HardLineBreak() {
			b.writeString("\n")
		} else if n.SoftLineBreak() {
			b.writeString("\n")
		}

	case *ast.String:
		b.writeString(string(n.Value))

	case *ast.RawHTML:
		// Render raw HTML segments as plain text
		segs := n.Segments
		for i := 0; i < segs.Len(); i++ {
			seg := segs.At(i)
			b.writeString(string(seg.Value(source)))
		}
		return

	default:
		// Handle extension types by kind
		switch {
		case n.Kind() == east.KindStrikethrough:
			start := b.utf16Off
			b.walkChildren(node, source)
			if length := b.utf16Off - start; length > 0 {
				b.entities = append(b.entities, tele.MessageEntity{
					Type:   tele.EntityStrikethrough,
					Offset: start,
					Length: length,
				})
			}
		case n.Kind() == gteast.KindUnderline:
			start := b.utf16Off
			b.walkChildren(node, source)
			if length := b.utf16Off - start; length > 0 {
				b.entities = append(b.entities, tele.MessageEntity{
					Type:   tele.EntityUnderline,
					Offset: start,
					Length: length,
				})
			}
		default:
			// Unknown node — walk children to preserve text content
			b.walkChildren(node, source)
		}
	}
}

func (b *entityBuilder) walkChildren(node ast.Node, source []byte) {
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		b.walkNode(c, source)
	}
}

// utf16Len returns the length of s in UTF-16 code units, which is
// the unit Telegram uses for entity offsets and lengths.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2 // surrogate pair
		} else {
			n++
		}
	}
	return n
}
