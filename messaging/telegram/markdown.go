package telegram

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	// Packages
	goldmark "github.com/yuin/goldmark"
	ast "github.com/yuin/goldmark/ast"
	extension "github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	text "github.com/yuin/goldmark/text"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// part is a message, as Telegram's HTML, and as plain text, which is sent
// when Telegram can't parse its HTML
type part struct {
	html  string
	plain string
}

// renderer writes Markdown's blocks as Telegram's HTML, which has bold,
// italic, strikethrough, code, preformatted text, links and quotes, but no
// headings, lists or tables
type renderer struct {
	source []byte
	b      strings.Builder
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))
	escaper  = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	attr     = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	reTag    = regexp.MustCompile(`<[^>]*>`)
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// parts returns the Markdown as messages of Telegram's HTML, each of at
// most n characters, broken between blocks, such as paragraphs, and a
// block which is longer, broken as plain text
func parts(markdownText string, n int) []part {
	source := []byte(markdownText)
	doc := markdown.Parser().Parse(text.NewReader(source))

	var result []part
	var current []string
	length := 0
	flush := func() {
		if len(current) > 0 {
			html := strings.Join(current, "\n\n")
			result = append(result, part{html: html, plain: plainText(html)})
			current, length = nil, 0
		}
	}
	for block := doc.FirstChild(); block != nil; block = block.NextSibling() {
		r := &renderer{source: source}
		r.block(block, 0)
		html := strings.TrimSpace(r.b.String())
		if html == "" {
			continue
		}
		size := utf8.RuneCountInString(html)
		switch {
		case size > n:
			// It's too long for a message, so it's sent as plain text
			flush()
			for _, s := range split(plainText(html), n) {
				result = append(result, part{html: escaper.Replace(s), plain: s})
			}
		case length > 0 && length+2+size > n:
			flush()
			fallthrough
		default:
			current = append(current, html)
			length += size + 2
		}
	}
	flush()
	return result
}

// plainText returns the HTML's text, without its tags
func plainText(s string) string {
	return html.UnescapeString(reTag.ReplaceAllString(s, ""))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - BLOCKS

// block writes the block, in a list nested to the depth
func (r *renderer) block(node ast.Node, depth int) {
	switch n := node.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		r.inlines(n)
	case *ast.Heading:
		r.b.WriteString("<b>")
		r.inlines(n)
		r.b.WriteString("</b>")
	case *ast.ThematicBreak:
		r.b.WriteString("———")
	case *ast.FencedCodeBlock:
		if language := strings.TrimSpace(string(n.Language(r.source))); language != "" {
			r.b.WriteString(`<pre><code class="language-` + attr.Replace(language) + `">`)
		} else {
			r.b.WriteString("<pre><code>")
		}
		r.lines(n)
		r.b.WriteString("</code></pre>")
	case *ast.CodeBlock:
		r.b.WriteString("<pre><code>")
		r.lines(n)
		r.b.WriteString("</code></pre>")
	case *ast.Blockquote:
		r.b.WriteString("<blockquote>")
		r.children(n, depth, "\n")
		r.b.WriteString("</blockquote>")
	case *ast.List:
		r.list(n, depth)
	case *ast.HTMLBlock:
		r.lines(n)
		if n.HasClosure() {
			r.b.WriteString(escaper.Replace(string(n.ClosureLine.Value(r.source))))
		}
	case *east.Table:
		r.table(n)
	default:
		r.children(n, depth, "\n\n")
	}
}

// children writes the node's blocks, with the separator between them
func (r *renderer) children(node ast.Node, depth int, sep string) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if child != node.FirstChild() {
			r.b.WriteString(sep)
		}
		r.block(child, depth)
	}
}

// list writes the list's items, each on a line, with a bullet, or its
// number, indented to its depth, with the lists in them indented further
func (r *renderer) list(list *ast.List, depth int) {
	number := list.Start
	if number == 0 {
		number = 1
	}
	indent := strings.Repeat("    ", depth)
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		if item != list.FirstChild() {
			r.b.WriteString("\n")
		}
		r.b.WriteString(indent)
		if list.IsOrdered() {
			r.b.WriteString(strconv.Itoa(number) + ". ")
			number++
		} else {
			r.b.WriteString("• ")
		}
		for child := item.FirstChild(); child != nil; child = child.NextSibling() {
			if _, nested := child.(*ast.List); nested {
				r.b.WriteString("\n")
				r.block(child, depth+1)
				continue
			} else if child != item.FirstChild() {
				r.b.WriteString("\n" + indent + "  ")
			}
			r.block(child, depth)
		}
	}
}

// table writes the table as preformatted text, whose columns line up, as
// Telegram has no tables
func (r *renderer) table(table *east.Table) {
	var rows [][]string
	var widths []int
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			value := strings.TrimSpace(r.plain(cell))
			if i := len(cells); i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[len(cells)] = max(widths[len(cells)], utf8.RuneCountInString(value))
			cells = append(cells, value)
		}
		rows = append(rows, cells)
	}
	var b strings.Builder
	for i, cells := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		for j, cell := range cells {
			if j > 0 {
				b.WriteString(" │ ")
			}
			b.WriteString(cell)
			if j < len(cells)-1 {
				b.WriteString(strings.Repeat(" ", widths[j]-utf8.RuneCountInString(cell)))
			}
		}
		if i == 0 {
			// The header's line
			b.WriteString("\n")
			for j, width := range widths {
				if j > 0 {
					b.WriteString("─┼─")
				}
				b.WriteString(strings.Repeat("─", width))
			}
		}
	}
	r.b.WriteString("<pre>" + escaper.Replace(b.String()) + "</pre>")
}

// lines writes the block's lines, as they are, such as code
func (r *renderer) lines(node ast.Node) {
	var b strings.Builder
	lines := node.Lines()
	for i := range lines.Len() {
		segment := lines.At(i)
		b.Write(segment.Value(r.source))
	}
	r.b.WriteString(escaper.Replace(strings.TrimRight(b.String(), "\n")))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - INLINES

// inlines writes the node's inlines
func (r *renderer) inlines(node ast.Node) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		r.inline(child)
	}
}

// inline writes the inline, with its tags
func (r *renderer) inline(node ast.Node) {
	switch n := node.(type) {
	case *ast.Text:
		r.b.WriteString(escaper.Replace(string(n.Value(r.source))))
		if n.SoftLineBreak() || n.HardLineBreak() {
			r.b.WriteString("\n")
		}
	case *ast.String:
		r.b.WriteString(escaper.Replace(string(n.Value)))
	case *ast.Emphasis:
		tag := "i"
		if n.Level >= 2 {
			tag = "b"
		}
		r.b.WriteString("<" + tag + ">")
		r.inlines(n)
		r.b.WriteString("</" + tag + ">")
	case *east.Strikethrough:
		r.b.WriteString("<s>")
		r.inlines(n)
		r.b.WriteString("</s>")
	case *ast.CodeSpan:
		r.b.WriteString("<code>" + escaper.Replace(r.plain(n)) + "</code>")
	case *ast.Link:
		r.b.WriteString(`<a href="` + attr.Replace(string(n.Destination)) + `">`)
		r.inlines(n)
		r.b.WriteString("</a>")
	case *ast.AutoLink:
		url := string(n.URL(r.source))
		if n.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(url, "mailto:") {
			url = "mailto:" + url
		}
		r.b.WriteString(`<a href="` + attr.Replace(url) + `">` + escaper.Replace(string(n.Label(r.source))) + "</a>")
	case *ast.Image:
		label := strings.TrimSpace(r.plain(n))
		if label == "" {
			label = "Image"
		}
		r.b.WriteString(`<a href="` + attr.Replace(string(n.Destination)) + `">` + escaper.Replace(label) + "</a>")
	case *ast.RawHTML:
		for i := range n.Segments.Len() {
			segment := n.Segments.At(i)
			r.b.WriteString(escaper.Replace(string(segment.Value(r.source))))
		}
	case *east.TaskCheckBox:
		if n.IsChecked {
			r.b.WriteString("☑ ")
		} else {
			r.b.WriteString("☐ ")
		}
	default:
		r.inlines(n)
	}
}

// plain returns the text of the node's inlines, without tags, or escaping
func (r *renderer) plain(node ast.Node) string {
	inner := &renderer{source: r.source}
	inner.inlines(node)
	return plainText(inner.b.String())
}
