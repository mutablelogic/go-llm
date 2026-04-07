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
	var opts opts
	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}

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
	return &markdown{renderer: renderer}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *markdown) Write(w io.Writer, text string) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return io.WriteString(w, "")
	}
	if m == nil || m.renderer == nil {
		return io.WriteString(w, text)
	}

	out, err := m.renderer.Render(text)
	if err != nil {
		return io.WriteString(w, text)
	}

	return io.WriteString(w, strings.TrimSpace(out))
}
