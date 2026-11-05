package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/memory/httpclient"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemoryCommands are the commands which list, get, remember and forget the
// memories
type MemoryCommands struct {
	Memories       MemoriesCmd       `cmd:"" name:"memories" help:"List what's remembered, or search it, the most recent first." group:"MEMORY"`
	Memory         MemoryCmd         `cmd:"" name:"memory" help:"Print what's remembered under a key." group:"MEMORY"`
	MemoryRemember MemoryRememberCmd `cmd:"" name:"memory-remember" help:"Remember something under a key, from the arguments, or standard input." group:"MEMORY"`
	MemoryForget   MemoryForgetCmd   `cmd:"" name:"memory-forget" help:"Forget what's remembered under a key, or a line of it, with --line." group:"MEMORY"`
}

type MemoriesCmd struct {
	Q      []string `arg:"" name:"words" optional:"" help:"Words which each memory has, in its key, description or body, or every memory."`
	Offset uint64   `name:"offset" help:"How many memories to skip."`
	Limit  *uint64  `name:"limit" help:"Most memories to list."`
}

type MemoryCmd struct {
	Key string `arg:"" name:"key" help:"Key of the memory, such as kitchen_lights."`
}

type MemoryRememberCmd struct {
	Key         string   `arg:"" name:"key" help:"Key of the memory, such as kitchen_lights."`
	Body        []string `arg:"" name:"body" optional:"" help:"What's remembered, as Markdown, which can be more than one argument, after --, when it starts with a bullet, such as -- '- Otto', or is read from standard input."`
	Description string   `name:"description" short:"d" required:"" help:"A line which describes what's remembered."`
	Version     string   `name:"at" help:"Version of the memory it replaces, which memory prints, or none, to remember a new one."`
}

type MemoryForgetCmd struct {
	Key  string `arg:"" name:"key" help:"Key of the memory, such as kitchen_lights."`
	Line string `name:"line" help:"A line of the memory to forget, rather than the whole memory."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the memories, as a table, and how many there are
func (cmd *MemoriesCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "memory.MemoriesCmd", func(ctx context.Context, client *httpclient.Client) error {
		list, err := client.ListMemories(ctx, schema.MemoryListRequest{Q: strings.Join(cmd.Q, " "), Offset: cmd.Offset, Limit: cmd.Limit})
		if err != nil {
			return err
		} else if debug {
			fmt.Println(list)
			return nil
		}
		if _, err := tui.TableFor[schema.Memory](tui.SetWidth(width)).Write(os.Stdout, list.Body...); err != nil {
			return err
		}
		_, err = tui.TableSummary("memories", list.Count, uint(len(list.Body)), cmd.Offset, cmd.Limit).Write(os.Stdout)
		return err
	})
}

// Run prints the memory
func (cmd *MemoryCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "memory.MemoryCmd", func(ctx context.Context, client *httpclient.Client) error {
		memory, err := client.GetMemory(ctx, cmd.Key)
		if err != nil {
			return err
		}
		return printMemory(memory, debug)
	})
}

// Run remembers the memory, and prints it
func (cmd *MemoryRememberCmd) Run(ctx server.Cmd) error {
	body := strings.Join(cmd.Body, " ")
	if strings.TrimSpace(body) == "" {
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return fmt.Errorf("missing what's remembered, as arguments, or on standard input")
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		body = string(data)
	}
	debug := ctx.IsDebug()
	return withClient(ctx, "memory.MemoryRememberCmd", func(ctx context.Context, client *httpclient.Client) error {
		memory, err := client.RememberMemory(ctx, cmd.Key, schema.MemoryRequest{Description: cmd.Description, Body: body, Version: cmd.Version})
		if err != nil {
			return err
		}
		return printMemory(memory, debug)
	})
}

// Run forgets the memory, or a line of it, and prints it, as it is, or was
func (cmd *MemoryForgetCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "memory.MemoryForgetCmd", func(ctx context.Context, client *httpclient.Client) error {
		memory, err := client.ForgetMemory(ctx, cmd.Key, cmd.Line)
		if err != nil {
			return err
		}
		return printMemory(memory, debug)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// printMemory prints the memory's key, description, version, and when it
// was last changed, and then its body, or, with debugging, the memory as it
// is
func printMemory(memory *schema.Memory, debug bool) error {
	if debug {
		fmt.Println(memory)
		return nil
	}
	updated := memory.Created
	if memory.Modified.After(updated) {
		updated = memory.Modified
	}
	fmt.Printf("%s: %s\n", memory.Key, memory.Description)
	fmt.Printf("Version %s, updated %s\n\n", memory.Version, updated.Local().Format(time.DateTime))
	fmt.Println(memory.Body)
	return nil
}
