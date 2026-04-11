package tui

import (
	"io"
	"strings"

	// Packages
	glamour "github.com/charmbracelet/glamour"
	termenv "github.com/muesli/termenv"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type markdown struct {
	renderer *glamour.TermRenderer
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func Markdown(options ...Opt) *markdown {
	return &markdown{renderer: newMarkdownRenderer(applyOpts(options...))}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *markdown) Write(w io.Writer, text string) (int, error) {
	out, err := renderMarkdown(m.renderer, text)
	if out == "" {
		return io.WriteString(w, "")
	}
	if err != nil {
		return io.WriteString(w, strings.TrimSpace(text))
	}

	return io.WriteString(w, strings.TrimSpace(out))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func applyOpts(options ...Opt) opts {
	var opts opts
	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}
	return opts
}

func newMarkdownRenderer(opts opts) *glamour.TermRenderer {
	stylePath := "dark"
	if !termenv.HasDarkBackground() {
		stylePath = "light"
	}

	rendererOptions := []glamour.TermRendererOption{
		glamour.WithStylePath(stylePath),
	}
	if opts.width > 0 {
		rendererOptions = append(rendererOptions, glamour.WithWordWrap(opts.width))
	}

	renderer, _ := glamour.NewTermRenderer(rendererOptions...)
	return renderer
}

func renderMarkdown(renderer *glamour.TermRenderer, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if renderer == nil {
		return text, nil
	}

	out, err := renderer.Render(text)
	if err != nil {
		return text, err
	}

	return strings.TrimSpace(out), nil
}
