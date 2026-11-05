package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	// Packages
	ansi "github.com/charmbracelet/x/ansi"
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
	term "golang.org/x/term"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type AskCmd struct {
	Text   []string                   `arg:"" name:"text" help:"The question to ask, which can be more than one argument."`
	Files  []string                   `name:"file" short:"f" sep:"none" help:"A file the question is about, which can be given more than once. Text, such as code or subtitles, is sent as text, with its name, and anything else, such as an image or a PDF, as an attachment, which the model must accept." optional:""`
	Model  string                     `name:"model" help:"Model name, or provider and model name separated by a colon, such as ollama:llama3.2:latest. Defaults to the model last asked." optional:""`
	System string                     `name:"system" help:"A system prompt, such as instructions on how to reply." optional:""`
	Tools  []string                   `name:"tool" help:"Name of a tool the model can call, such as weatherapi:current, or weatherapi:* for all the tools in a namespace, which can be given more than once. The model's calls are printed, rather than run." optional:""`
	Opt    map[string]json.RawMessage `name:"opt" help:"Options, by key, such as temperature=0.7, which can be given more than once." optional:""`
	Schema json.RawMessage            `name:"schema" help:"JSON schema of the reply, which is printed as JSON." optional:""`
	Stream bool                       `name:"stream" negatable:"" default:"true" help:"Print the reply as it's generated, or with --no-stream, once it's complete, rendered as markdown on a terminal."`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultModelKey is the key of the default model, which is the model last
// asked
const defaultModelKey = "llm.model"

// errNoResult is the error when the reply has a schema, but the model didn't
// reply with JSON which conforms to it
var errNoResult = errors.New("the model didn't reply with JSON which matches the schema")

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the model's reply to the question, as it's generated, or once
// it's complete, and the tools it called, or its reply as JSON, with a
// schema, and keeps the model, by its canonical name, as the default
func (cmd *AskCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	width := ctx.IsTerm()

	// The model, or the model last asked
	model := strings.TrimSpace(cmd.Model)
	if model == "" {
		model = strings.TrimSpace(ctx.GetString(defaultModelKey))
	}
	if model == "" {
		return fmt.Errorf("missing model: set one with --model, and it's used until another is set")
	}

	// The question
	text := strings.TrimSpace(strings.Join(cmd.Text, " "))
	if text == "" {
		return fmt.Errorf("missing question")
	}
	message, err := askMessage(text, cmd.Files)
	if err != nil {
		return err
	}

	return withClient(ctx, "llm.AskCmd", func(reqctx context.Context, client *httpclient.Client) error {
		// A reply with a schema is JSON, so isn't streamed
		printer := newStreamPrinter(debug || !cmd.Stream || len(cmd.Schema) > 0, width)
		response, err := client.Ask(reqctx, schema.AskRequest{
			Model:    types.Ptr(model),
			System:   types.Ptr(strings.TrimSpace(cmd.System)),
			Tools:    cmd.Tools,
			Opt:      quoteOpts(cmd.Opt),
			Schema:   cmd.Schema,
			Messages: []providerschema.MessageMeta{message.MessageMeta},
		}, printer.Stream())
		printer.End()
		if err != nil {
			return err
		}

		// The model is the default once it's replied, by its canonical name,
		// as its name could match another provider's model later
		if err := ctx.Set(defaultModelKey, response.Model); err != nil {
			return err
		}

		// With debugging, the response
		if debug {
			fmt.Println(response)
			return nil
		}

		// The reply with a schema is JSON
		if len(response.Result) > 0 {
			return printJSON(response.Result)
		}

		// Otherwise the reply's text, unless it was streamed, and the tools
		// it called
		if text := response.Message.Text(); text != "" && !printer.Streamed() {
			if err := printText(width, text); err != nil {
				return err
			}
		}
		for _, block := range response.Message.Content {
			if call := block.ToolCall; call != nil {
				fmt.Printf("%s(%s)\n", call.Name, string(call.Input))
			}
		}
		if response.Schema {
			return errNoResult
		}
		return nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// askMessage returns the message which asks the question about the files:
// the text of each file which is text, with its name, and each other file as
// an attachment, and then the question
func askMessage(text string, files []string) (*providerschema.Message, error) {
	message, err := providerschema.NewMessage("user", text)
	if err != nil {
		return nil, err
	}
	content := make([]providerschema.ContentBlock, 0, len(files)+1)
	for _, file := range files {
		block, err := fileBlock(file)
		if err != nil {
			return nil, err
		}
		content = append(content, block)
	}
	message.Content = append(content, message.Content...)
	return message, nil
}

// fileBlock returns the content of a file, as text, with its name, in a
// fenced code block, when it's text, or otherwise as an attachment, with its
// type
func fileBlock(file string) (providerschema.ContentBlock, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return providerschema.ContentBlock{}, err
	}
	name := filepath.Base(file)
	contentType := providerschema.ResourceType(data, name)
	if isText(data, contentType) {
		text := strings.TrimRight(string(data), "\n")
		fence := codeFence(text)
		return providerschema.ContentBlock{Text: types.Ptr(fmt.Sprintf("%s:\n\n%s\n%s\n%s", name, fence, text, fence))}, nil
	} else if contentType == "" {
		return providerschema.ContentBlock{}, fmt.Errorf("%s: its type isn't known", file)
	}
	return providerschema.ContentBlock{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{
		ContentType: contentType,
		Data:        data,
	}}}, nil
}

// codeFence returns the fence of a code block of the text, of backticks,
// which is longer than any run of backticks in the text, and at least
// three, so a fenced code block in the text, such as of a Markdown file,
// doesn't close it
func codeFence(text string) string {
	longest, run := 0, 0
	for _, r := range text {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

// isText returns true when data is text, which is UTF-8, without NUL bytes,
// and isn't a type, such as a PDF or an image, which is sent as it is, even
// when it's UTF-8
func isText(data []byte, contentType string) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasPrefix(mediaType, "image/"), strings.HasPrefix(mediaType, "audio/"), strings.HasPrefix(mediaType, "video/"), mediaType == "application/pdf":
		return false
	case len(data) == 0:
		return true
	}
	return utf8.Valid(data) && !bytes.ContainsRune(data, 0)
}

// printJSON prints the JSON, indented
func printJSON(data json.RawMessage) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	_, err := os.Stdout.Write(buf.Bytes())
	return err
}

// printText prints a model's reply, rendered as markdown to fit the width of
// the terminal, or as it is when the output isn't a terminal, when the width
// is zero, with a newline only when it doesn't end with one
func printText(width int, text string) error {
	if width <= 0 {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		_, err := io.WriteString(os.Stdout, text)
		return err
	}
	_, err := tui.MarkdownFormatter(tui.SetWidth(width)).Write(os.Stdout, text)
	return err
}

///////////////////////////////////////////////////////////////////////////////
// STREAM PRINTER

// streamPrinter prints a model's reply as it's generated. Its text is
// printed as it is, and on a terminal, once it's complete, it's printed again,
// in place of what was printed, rendered as markdown, and its thinking is
// shown as "Thinking…", which is replaced too. What tools log is printed,
// dimmed, and kept. Text which is taller than the terminal isn't printed
// again, as what's scrolled off it can't be replaced.
type streamPrinter struct {
	w        io.Writer
	width    int                 // The width of the terminal, or zero when the output isn't one
	height   int                 // The height of the terminal, or zero when it isn't known
	live     strings.Builder     // What's printed which is to be replaced
	text     strings.Builder     // The text which replaces it, rendered as markdown
	thinking bool                // Whether "Thinking…" is printed, which is replaced
	last     llm.StreamType      // What was printed last
	trailing string              // The last piece of text which was printed
	render   func(string) string // Renders text as markdown, to fit the width
}

// thinkingLine is printed for the model's thinking, on a terminal, which is
// replaced by the text which follows
const thinkingLine = "Thinking…"

// newStreamPrinter returns a printer of a reply as it's generated, or nil
// when the reply isn't streamed. The width is the width of the terminal, or
// zero when the output isn't a terminal.
func newStreamPrinter(off bool, width int) *streamPrinter {
	if off {
		return nil
	}
	p := &streamPrinter{w: os.Stdout, width: width}
	if width > 0 {
		if _, height, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			p.height = height
		}
		p.render = func(text string) string {
			var b strings.Builder
			if _, err := tui.MarkdownFormatter(tui.SetWidth(width)).Write(&b, text); err != nil {
				return text
			}
			return b.String()
		}
	}
	return p
}

// Stream returns the function which prints each piece of the reply, or nil
// when the reply isn't streamed
func (p *streamPrinter) Stream() llm.GeneratorStreamFn {
	if p == nil {
		return nil
	}
	return p.print
}

// Streamed returns true when the reply's text was printed as it was
// generated
func (p *streamPrinter) Streamed() bool {
	return p != nil
}

// End prints the text again, rendered as markdown, on a terminal, and ends
// what's printed with a newline, when it doesn't end with one
func (p *streamPrinter) End() {
	if p == nil {
		return
	}
	p.replace()
	if p.last != "" && !strings.HasSuffix(p.trailing, "\n") {
		fmt.Fprintln(p.w)
	}
}

func (p *streamPrinter) print(what llm.StreamType, text string) {
	if text == "" {
		return
	}
	switch {
	case p.width <= 0:
		// Not a terminal: only the text, as it is
		if what != llm.StreamText {
			return
		}
		p.write(what, text, false)
	case what == llm.StreamThinking:
		// That the model is thinking, once, which is replaced
		if !p.thinking {
			p.thinking = true
			p.write(what, "\x1b[2m"+thinkingLine+"\x1b[0m\n", true)
		}
	case what == llm.StreamLog:
		// What's printed before it is replaced first, so it's kept after it,
		// and it's on a line of its own, even when what's before it isn't
		// replaced
		p.replace()
		line := "\x1b[2m" + strings.TrimRight(text, "\n") + "\x1b[0m\n"
		if p.last != "" && !strings.HasSuffix(p.trailing, "\n") {
			line = "\n" + line
		}
		p.write(what, line, false)
	default:
		p.text.WriteString(text)
		p.write(what, text, true)
	}
}

// write prints the text, with a blank line before it, when it's of another
// kind than what was printed last, apart from a line which is logged, and
// keeps it to be replaced, when it's live
func (p *streamPrinter) write(what llm.StreamType, text string, live bool) {
	if p.last != "" && p.last != what && p.last != llm.StreamLog && what != llm.StreamLog {
		text = strings.Repeat("\n", 2-min(2, newlines(p.trailing))) + text
	}
	p.last, p.trailing = what, text
	if live {
		p.live.WriteString(text)
	}
	fmt.Fprint(p.w, text)
}

// replace replaces what's printed to be replaced, which is the text, as it
// is, and that the model is thinking, with the text, rendered as markdown,
// when it fits on the terminal
func (p *streamPrinter) replace() {
	defer func() {
		p.live.Reset()
		p.text.Reset()
		p.thinking = false
	}()
	if p.render == nil || p.live.Len() == 0 {
		return
	}
	// Text with tabs isn't replaced, as how far they move isn't known, nor
	// is text which is as tall as the terminal, as what's scrolled off it
	// can't be
	if strings.Contains(p.live.String(), "\t") {
		return
	}
	rows := rowsOf(p.live.String(), p.width)
	if p.height > 0 && rows >= p.height {
		return
	}

	// Move to the start of the first row which was printed, and clear to the
	// end of the screen, and print the text, rendered, apart from the blank
	// lines around it
	fmt.Fprintf(p.w, "\r")
	if rows > 1 {
		fmt.Fprintf(p.w, "\x1b[%dA", rows-1)
	}
	fmt.Fprint(p.w, "\x1b[J")
	text := ""
	if strings.TrimSpace(p.text.String()) != "" {
		text = strings.Trim(p.render(p.text.String()), "\n") + "\n"
	}
	fmt.Fprint(p.w, text)
	p.last, p.trailing = llm.StreamText, text
}

// rowsOf returns how many rows of the terminal, of the width, the text is
// printed on, from the row it starts on, to the row the cursor's on, after
// it's printed
func rowsOf(text string, width int) int {
	rows := 0
	for _, line := range strings.Split(ansi.Strip(text), "\n") {
		rows += max(1, (ansi.StringWidth(line)+width-1)/width)
	}
	return rows
}

// newlines returns how many newlines the text ends with
func newlines(text string) int {
	return len(text) - len(strings.TrimRight(text, "\n"))
}
