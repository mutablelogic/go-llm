package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/openai"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type CompleteCmd struct {
	Prompt      string   `arg:"" optional:"" help:"Prompt"`
	Model       string   `flag:"model" help:"Model name"`
	File        []string `type:"file" short:"f" help:"Files to attach"`
	System      string   `flag:"system" help:"Set the system prompt"`
	NoStream    bool     `flag:"no-stream" help:"Do not stream output"`
	Format      string   `flag:"format" enum:"text,markdown,json,image,audio" default:"text" help:"Output format"`
	Size        string   `flag:"size" enum:"256x256,512x512,1024x1024,1792x1024,1024x1792" default:"1024x1024" help:"Image size"`
	Style       string   `flag:"style" enum:"vivid,natural" default:"vivid" help:"Image style"`
	Quality     string   `flag:"quality" enum:"standard,hd" default:"standard" help:"Image quality"`
	Temperature *float64 `flag:"temperature" short:"t"  help:"Temperature for sampling"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func typeFromFormat(format string) Type {
	switch format {
	case "image":
		return ImageType
	case "audio":
		return AudioType
	default:
		return TextType
	}
}

func (cmd *CompleteCmd) Run(globals *Globals) error {
	return run(globals, typeFromFormat(cmd.Format), cmd.Model, func(ctx context.Context, model llm.Model) error {
		var prompt []byte

		// If we are pipeline content in via stdin
		fileInfo, err := os.Stdin.Stat()
		if err != nil {
			return llm.ErrInternalServerError.Withf("Failed to get stdin stat: %v", err)
		}
		if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
			if data, err := io.ReadAll(os.Stdin); err != nil {
				return err
			} else if len(data) > 0 {
				prompt = data
			}
		}

		// Append any further prompt
		if len(cmd.Prompt) > 0 {
			prompt = append(prompt, []byte("\n\n")...)
			prompt = append(prompt, []byte(cmd.Prompt)...)
		}

		opts := cmd.opts()
		if !cmd.NoStream {
			// Add streaming callback
			var buf string
			opts = append(opts, llm.WithStream(func(c llm.Completion) {
				fmt.Print(strings.TrimPrefix(c.Text(0), buf))
				buf = c.Text(0)
			}))
		}

		// Add attachments
		for _, file := range cmd.File {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()
			opts = append(opts, llm.WithAttachment(f))
		}

		// Make the completion
		completion, err := model.Completion(ctx, string(prompt), opts...)
		if err != nil {
			return err
		}

		// Print the completion - text
		if cmd.NoStream {
			fmt.Println(completion.Text(0))
		}

		// Output completion attachments
		for i := 0; i < completion.Num(); i++ {
			attachment := completion.Attachment(i)
			if attachment == nil {
				continue
			}
			if attachment.Filename() == "" {
				continue
			}
			f, err := os.Create(attachment.Filename())
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := f.Write(attachment.Data()); err != nil {
				return err
			} else {
				fmt.Printf("%q written to %s\n", attachment.Caption(), attachment.Filename())
			}
		}

		// Return success
		return nil
	})
}

func (cmd *CompleteCmd) opts() []llm.Opt {
	opts := []llm.Opt{}

	// Set system prompt
	var system []string
	if cmd.Format == "markdown" {
		system = append(system, "Structure your output in markdown format.")
	} else if cmd.Format == "json" {
		system = append(system, "Structure your output in JSON format.")
	}
	if cmd.System != "" {
		system = append(system, cmd.System)
	}
	if len(system) > 0 {
		opts = append(opts, llm.WithSystemPrompt(strings.Join(system, "\n")))
	}

	// Set format
	opts = append(opts, llm.WithFormat(cmd.Format))

	// Set image parameters
	opts = append(opts, openai.WithSize(cmd.Size))
	opts = append(opts, openai.WithStyle(cmd.Style))
	opts = append(opts, openai.WithQuality(cmd.Quality))

	// Set temperature
	if cmd.Temperature != nil {
		opts = append(opts, llm.WithTemperature(*cmd.Temperature))
	}

	return opts
}
