package tui

import (
	"bytes"
	"strings"
	"testing"
)

func TestMarkdownWrite(t *testing.T) {
	widget := Markdown(SetWidth(40))
	var buffer bytes.Buffer

	if _, err := widget.Write(&buffer, "# Title\n\nA **bold** paragraph.\n\n- One\n- Two"); err != nil {
		t.Fatal(err)
	}

	out := buffer.String()
	if !strings.Contains(out, "Title") {
		t.Fatalf("expected heading in output, got %q", out)
	}
	if !strings.Contains(out, "bold") {
		t.Fatalf("expected paragraph content in output, got %q", out)
	}
	if !strings.Contains(out, "One") || !strings.Contains(out, "Two") {
		t.Fatalf("expected list items in output, got %q", out)
	}
}

func TestMarkdownWriteEmpty(t *testing.T) {
	widget := Markdown(SetWidth(40))
	var buffer bytes.Buffer

	if _, err := widget.Write(&buffer, "   "); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
}
