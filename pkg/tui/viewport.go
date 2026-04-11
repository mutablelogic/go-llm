package tui

import (
	"strings"

	// Packages
	glamour "github.com/charmbracelet/glamour"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Viewport struct {
	renderer *glamour.TermRenderer
	width    int
	height   int
	entries  []viewportEntry
	cursor   viewportCursor
	lines    []string
	offset   int
	index    map[string]int
}

type viewportEntry struct {
	key  string
	text string
}

type viewportCursor struct {
	key     string
	text    string
	visible bool
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewViewport(options ...Opt) *Viewport {
	opts := applyOpts(options...)
	return &Viewport{
		renderer: newMarkdownRenderer(opts),
		width:    opts.width,
		height:   opts.height,
		index:    make(map[string]int),
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (v *Viewport) Append(text string) error {
	if text == "" {
		return nil
	}
	v.entries = append(v.entries, viewportEntry{text: text})
	return v.refresh()
}

func (v *Viewport) SetContent(text string) error {
	v.entries = nil
	v.index = make(map[string]int)
	if text != "" {
		v.entries = append(v.entries, viewportEntry{text: text})
	}
	return v.refresh()
}

func (v *Viewport) SetSection(key, text string) error {
	if key == "" {
		return v.SetContent(text)
	}
	if idx, ok := v.index[key]; ok {
		if text == "" {
			v.removeEntry(idx)
		} else {
			v.entries[idx].text = text
		}
		return v.refresh()
	}
	if text == "" {
		return nil
	}
	v.entries = append(v.entries, viewportEntry{key: key, text: text})
	v.index[key] = len(v.entries) - 1
	return v.refresh()
}

func (v *Viewport) AppendSection(key, text string) error {
	if key == "" {
		return v.Append(text)
	}
	if text == "" {
		return nil
	}
	if idx, ok := v.index[key]; ok {
		v.entries[idx].text += text
		return v.refresh()
	}
	v.entries = append(v.entries, viewportEntry{key: key, text: text})
	v.index[key] = len(v.entries) - 1
	return v.refresh()
}

func (v *Viewport) DeleteSection(key string) error {
	if key == "" {
		return v.SetContent("")
	}
	if idx, ok := v.index[key]; ok {
		v.removeEntry(idx)
		return v.refresh()
	}
	return v.refresh()
}

func (v *Viewport) SetCursor(key, text string) error {
	if key == "" || text == "" {
		return v.ClearCursor()
	}
	visible := true
	if v.cursor.key != "" || v.cursor.text != "" {
		visible = v.cursor.visible
	}
	if v.cursor.key == key && v.cursor.text == text && v.cursor.visible == visible {
		return nil
	}
	v.cursor = viewportCursor{key: key, text: text, visible: visible}
	return v.refresh()
}

func (v *Viewport) SetCursorVisible(visible bool) error {
	if v.cursor.key == "" || v.cursor.text == "" || v.cursor.visible == visible {
		return nil
	}
	v.cursor.visible = visible
	return v.refresh()
}

func (v *Viewport) ClearCursor() error {
	if v.cursor.key == "" && v.cursor.text == "" {
		return nil
	}
	v.cursor = viewportCursor{}
	return v.refresh()
}

func (v *Viewport) Resize(width, height int) error {
	rebuild := false
	if width > 0 && width != v.width {
		v.width = width
		rebuild = true
	}
	if height > 0 {
		v.height = height
	}
	if rebuild {
		v.renderer = newMarkdownRenderer(opts{width: v.width, height: v.height})
	}
	return v.refresh()
}

func (v *Viewport) View() string {
	if len(v.lines) == 0 {
		return ""
	}
	if v.height <= 0 || v.height >= len(v.lines) {
		return strings.Join(v.lines, "\n")
	}
	start := clamp(v.offset, 0, v.maxOffset())
	end := min(start+v.height, len(v.lines))
	return strings.Join(v.lines[start:end], "\n")
}

func (v *Viewport) LineUp(n int) {
	if n <= 0 {
		n = 1
	}
	v.offset = max(0, v.offset-n)
}

func (v *Viewport) LineDown(n int) {
	if n <= 0 {
		n = 1
	}
	v.offset = min(v.maxOffset(), v.offset+n)
}

func (v *Viewport) PageUp() {
	v.LineUp(max(1, v.pageSize()))
}

func (v *Viewport) PageDown() {
	v.LineDown(max(1, v.pageSize()))
}

func (v *Viewport) GotoTop() {
	v.offset = 0
}

func (v *Viewport) GotoBottom() {
	v.offset = v.maxOffset()
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (v *Viewport) refresh() error {
	hadContent := len(v.lines) > 0
	wasBottom := v.offset >= v.maxOffset()
	rendered, err := renderMarkdown(v.renderer, v.compose())
	if rendered == "" {
		v.lines = nil
		v.offset = 0
		return err
	}
	v.lines = strings.Split(rendered, "\n")
	if hadContent && wasBottom {
		v.GotoBottom()
	} else {
		v.offset = clamp(v.offset, 0, v.maxOffset())
	}
	return err
}

func (v *Viewport) compose() string {
	parts := make([]string, 0, len(v.entries))
	for _, entry := range v.entries {
		text := entry.text
		if v.cursor.visible && v.cursor.key != "" && v.cursor.key == entry.key {
			text += v.cursor.text
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

func (v *Viewport) removeEntry(idx int) {
	key := v.entries[idx].key
	v.entries = append(v.entries[:idx], v.entries[idx+1:]...)
	if key != "" {
		delete(v.index, key)
	}
	for i := idx; i < len(v.entries); i++ {
		if key := v.entries[i].key; key != "" {
			v.index[key] = i
		}
	}
}

func (v *Viewport) maxOffset() int {
	if len(v.lines) == 0 || v.height <= 0 || len(v.lines) <= v.height {
		return 0
	}
	return len(v.lines) - v.height
}

func (v *Viewport) pageSize() int {
	if v.height > 0 {
		return v.height
	}
	return 1
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
