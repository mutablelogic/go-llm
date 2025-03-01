package term

import (
	"fmt"
	"io"
	"os"

	// Packages
	format "github.com/MichaelMure/go-term-text"
	color "github.com/fatih/color"
	term "golang.org/x/term"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Term struct {
	r  io.Reader
	fd int
	*term.Terminal
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewTerm(r io.Reader) (*Term, error) {
	t := new(Term)
	t.r = r

	// Set file descriptor
	if osf, ok := r.(*os.File); ok {
		t.fd = int(osf.Fd())
		if term.IsTerminal(t.fd) {
			t.Terminal = term.NewTerminal(osf, "")
		}
	}

	// Return success
	return t, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Returns the width and height of the terminal, or (0,0) if we aren't in
// a terminal
func (t *Term) Size() (int, int) {
	if t.Terminal != nil {
		if w, h, err := term.GetSize(t.fd); err == nil {
			return w, h
		}
	}
	// Unable to get the size
	return 0, 0
}

func (t *Term) Println(v ...any) {
	text := fmt.Sprint(v...)
	w, _ := t.Size()
	if w > 0 {
		text, _ = format.Wrap(text, w)
	}
	fmt.Fprintln(os.Stdout, text)
}

func (t *Term) ReadLine(prompt string) (string, error) {
	// Set terminal raw mode
	if t.Terminal != nil {
		state, err := term.MakeRaw(t.fd)
		if err != nil {
			return "", err
		}
		defer term.Restore(t.fd, state)
	}

	// Set the prompt with color
	if t.Terminal != nil {
		prompt = color.New(color.Bold).Sprint(prompt)
		t.Terminal.SetPrompt(prompt)
	}

	// Read the line
	if t.Terminal != nil {
		return t.Terminal.ReadLine()
	} else {
		// Don't support non-terminal input yet
		return "", io.EOF
	}
}
