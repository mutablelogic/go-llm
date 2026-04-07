package tui

import (
	"fmt"
	"io"
	"math"
	"strings"

	// Packages
	lipgloss "github.com/charmbracelet/lipgloss"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type progress struct {
	width   int
	label   lipgloss.Style
	filled  lipgloss.Style
	empty   lipgloss.Style
	percent lipgloss.Style
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func Progress(options ...Opt) *progress {
	var opts opts
	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}

	renderer := lipgloss.NewRenderer(nil)
	self := &progress{
		width: max(opts.width, 20),
	}
	self.label = renderer.NewStyle().Width(30)
	self.filled = renderer.NewStyle().Foreground(lipgloss.Color("10"))
	self.empty = renderer.NewStyle().Foreground(lipgloss.Color("8"))
	self.percent = renderer.NewStyle().Width(6).Align(lipgloss.Right)

	return self
}

func (p *progress) Write(w io.Writer, status string, percent float64) (int, error) {
	status = strings.TrimSpace(status)
	percent = clampPercent(percent)

	if percent <= 0 {
		if status == "" {
			return io.WriteString(w, "")
		}
		return io.WriteString(w, p.label.Render(status))
	}

	filled := min(int(math.Round((percent/100.0)*float64(p.width))), p.width)
	bar := p.filled.Render(strings.Repeat("█", filled)) + p.empty.Render(strings.Repeat("░", p.width-filled))
	if status == "" {
		return io.WriteString(w, fmt.Sprintf("[%s] %s", bar, p.percent.Render(fmt.Sprintf("%5.1f%%", percent))))
	}
	return io.WriteString(w, fmt.Sprintf("%s [%s] %s", p.label.Render(status), bar, p.percent.Render(fmt.Sprintf("%5.1f%%", percent))))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func clampPercent(percent float64) float64 {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}
