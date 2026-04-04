package tui

import (
	"io"

	// Packages
	lipgloss "github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

type TableRow interface {
	Header() []string
	Cell(int) string
	Width(int) int
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

type table[T TableRow] struct {
	table  *lgtable.Table
	widths []int
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func TableFor[T TableRow](options ...Opt) *table[T] {
	var opts opts
	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}

	renderer := lipgloss.NewRenderer(nil)
	self := new(table[T])
	renderedTable := lgtable.New().
		Border(lipgloss.NormalBorder()).
		Wrap(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := renderer.NewStyle()
			if col >= 0 && col < len(self.widths) && self.widths[col] > 0 {
				style = style.MaxWidth(self.widths[col])
			}
			if row == lgtable.HeaderRow {
				return style.Bold(true)
			}
			return style.Padding(0, 1)
		})
	if opts.width > 0 {
		renderedTable.Width(opts.width)
	}

	self.table = renderedTable
	return self
}

///////////////////////////////////////////////////////////////////////////////
// RENDER

func (t *table[T]) Write(w io.Writer, rows ...T) (int, error) {
	t.table.ClearRows()
	if len(rows) == 0 {
		return io.WriteString(w, "")
	}

	headers := rows[0].Header()
	t.widths = make([]int, len(headers))
	for i := range headers {
		t.widths[i] = rows[0].Width(i)
	}
	t.table.Headers(headers...)
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			cells[i] = row.Cell(i)
		}
		t.table.Row(cells...)
	}

	return io.WriteString(w, t.table.Render())
}
