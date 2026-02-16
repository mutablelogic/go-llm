// Package table provides a terminal table renderer backed by lipgloss.
// Consumers supply data via the TableData interface rather than building
// lipgloss tables directly.
package table

import (
	"fmt"
	"os"
	"strings"
	"time"

	// Packages
	lipgloss "github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	term "golang.org/x/term"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// TableData is the interface that data sources implement to be rendered
// as a terminal table.
type TableData interface {
	// Header returns the column header labels.
	Header() []string

	// Len returns the number of rows.
	Len() int

	// Row returns the cell values for row i. Values are converted to
	// strings via FormatCell. Return nil to skip a row.
	// Wrap a value in Bold{} to render it in bold.
	Row(i int) []any
}

// Bold wraps a cell value so that FormatCell renders it in bold.
type Bold struct{ Value any }

///////////////////////////////////////////////////////////////////////////////
// STYLES

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	boldStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	cellStyle   = lipgloss.NewStyle()
	dimStyle    = lipgloss.NewStyle().Faint(true)
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Render renders the table data as a string suitable for terminal output.
// Columns are auto-sized to the terminal width with word wrapping enabled.
func Render(data TableData) string {
	t := lgtable.New().
		Headers(data.Header()...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(dimStyle).
		Wrap(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == lgtable.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	for i := range data.Len() {
		row := data.Row(i)
		if row == nil {
			continue
		}
		cells := make([]string, len(row))
		for j, v := range row {
			cells[j] = FormatCell(v)
		}
		t.Row(cells...)
	}

	// Only constrain to terminal width if the natural render exceeds it
	result := t.Render()
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		// Check the widest line in the rendered output
		widest := 0
		for _, line := range strings.Split(result, "\n") {
			if n := len([]rune(line)); n > widest {
				widest = n
			}
		}
		if widest > w {
			t.Width(w)
			result = t.Render()
		}
	}

	return result
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// Truncate shortens s to max runes, collapsing newlines and appending "…"
// if truncated.
func Truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// FormatCell converts a value to a display string for a table cell.
// It handles nil, empty strings, zero values, time formatting, and Bold wrapping.
func FormatCell(v any) string {
	if v == nil {
		return "-"
	}
	switch val := v.(type) {
	case Bold:
		return boldStyle.Render(FormatCell(val.Value))
	case string:
		if val == "" {
			return "-"
		}
		return val
	case time.Time:
		if val.IsZero() {
			return "-"
		}
		return val.Format("2006-01-02 15:04")
	case int:
		if val == 0 {
			return "-"
		}
		return fmt.Sprint(val)
	case uint:
		if val == 0 {
			return "-"
		}
		return fmt.Sprint(val)
	default:
		s := fmt.Sprint(val)
		if s == "" {
			return "-"
		}
		return s
	}
}

// RenderMarkdown renders the table data as a Markdown table string,
// suitable for platforms that render markdown (Telegram, terminal with glamour).
func RenderMarkdown(data TableData) string {
	header := data.Header()
	if len(header) == 0 {
		return ""
	}
	var buf strings.Builder

	// Header row
	buf.WriteString("|")
	for _, h := range header {
		buf.WriteString(" ")
		buf.WriteString(h)
		buf.WriteString(" |")
	}
	buf.WriteString("\n|")
	for range header {
		buf.WriteString("---|")
	}

	// Data rows
	for i := range data.Len() {
		row := data.Row(i)
		if row == nil {
			continue
		}
		buf.WriteString("\n|")
		for j := range header {
			buf.WriteString(" ")
			if j < len(row) {
				buf.WriteString(formatMarkdownCell(row[j]))
			} else {
				buf.WriteString("-")
			}
			buf.WriteString(" |")
		}
	}
	return buf.String()
}

// formatMarkdownCell converts a cell value to a plain-text markdown cell string.
// Bold values are wrapped in ** markers.
func formatMarkdownCell(v any) string {
	if v == nil {
		return "-"
	}
	switch val := v.(type) {
	case Bold:
		inner := formatMarkdownCell(val.Value)
		if inner == "-" {
			return "-"
		}
		return "**" + inner + "**"
	case string:
		if val == "" {
			return "-"
		}
		return val
	case time.Time:
		if val.IsZero() {
			return "-"
		}
		return val.Format("2006-01-02 15:04")
	case int:
		if val == 0 {
			return "-"
		}
		return fmt.Sprint(val)
	case uint:
		if val == 0 {
			return "-"
		}
		return fmt.Sprint(val)
	default:
		s := fmt.Sprint(val)
		if s == "" {
			return "-"
		}
		return s
	}
}
