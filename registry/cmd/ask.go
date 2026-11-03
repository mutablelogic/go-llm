package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type AskCmd struct {
	Text   []string                   `arg:"" name:"text" help:"The question to ask, which can be more than one argument."`
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
	message, err := providerschema.NewMessage("user", text)
	if err != nil {
		return err
	}

	return withClient(ctx, "llm.AskCmd", func(reqctx context.Context, client *httpclient.Client) error {
		// A reply with a schema is JSON, so isn't streamed
		printer := newStreamPrinter(debug || !cmd.Stream || len(cmd.Schema) > 0, width)
		response, err := client.Ask(reqctx, schema.AskRequest{
			Model:    model,
			System:   strings.TrimSpace(cmd.System),
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

// streamPrinter prints a model's reply as it's generated: its text as it
// is, and on a terminal, its thinking, dimmed, with a blank line between
// them
type streamPrinter struct {
	width    int
	last     llm.StreamType
	trailing string
}

// newStreamPrinter returns a printer of a reply as it's generated, or nil
// when the reply isn't streamed. The width is the width of the terminal, or
// zero when the output isn't a terminal.
func newStreamPrinter(off bool, width int) *streamPrinter {
	if off {
		return nil
	}
	return &streamPrinter{width: width}
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

// End ends what's printed with a newline, when it doesn't end with one
func (p *streamPrinter) End() {
	if p != nil && p.last != "" && !strings.HasSuffix(p.trailing, "\n") {
		fmt.Println()
	}
}

func (p *streamPrinter) print(what llm.StreamType, text string) {
	if text == "" || (what == llm.StreamThinking && p.width <= 0) {
		return
	}

	// A blank line between the thinking and the text
	if p.last != "" && p.last != what {
		fmt.Print(strings.Repeat("\n", 2-min(2, newlines(p.trailing))))
	}
	p.last, p.trailing = what, text

	// The thinking is dimmed
	if what == llm.StreamThinking {
		fmt.Print("\x1b[2m" + text + "\x1b[0m")
	} else {
		fmt.Print(text)
	}
}

// newlines returns how many newlines the text ends with
func newlines(text string) int {
	return len(text) - len(strings.TrimRight(text, "\n"))
}
