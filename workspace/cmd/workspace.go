package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/workspace/httpclient"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// WorkspaceCommands list the workspaces, and their folders and resources
type WorkspaceCommands struct {
	Workspaces WorkspaceListCmd     `cmd:"" name:"workspaces" help:"List the workspaces, such as MCP connectors with resources." group:"WORKSPACES"`
	Workspace  WorkspaceGetCmd      `cmd:"" name:"workspace" help:"Get a workspace by name." group:"WORKSPACES"`
	Read       WorkspaceReadCmd     `cmd:"" name:"workspace-read" help:"Read a workspace's resource, by its URI, as it is, or as a Markdown document, with --format=markdown." group:"WORKSPACES"`
	Resources  WorkspaceResourceCmd `cmd:"" name:"workspace-list" help:"List a workspace's folders and resources, in a folder, or under it, with --recursive, a page at a time." group:"WORKSPACES"`
}

type WorkspaceListCmd struct{}

type WorkspaceGetCmd struct {
	Name string `arg:"" name:"name" help:"Name of the workspace, which is its namespace's."`
}

type WorkspaceResourceCmd struct {
	Name string `arg:"" name:"name" help:"Name of the workspace, which is its namespace's."`
	schema.ListRequest
}

type WorkspaceReadCmd struct {
	URI    string `arg:"" name:"uri" help:"URI of the resource, as it's listed, resource://<workspace>/<uri>."`
	Format string `name:"format" enum:"raw,markdown" default:"raw" help:"raw, for the resource as it is, or markdown, for its text, as a Markdown document, a part at a time."`
	Offset uint64 `name:"offset" help:"With markdown, the byte of its Markdown the part starts at, which is a part's next."`
	Limit  uint64 `name:"limit" help:"With markdown, the most bytes of its Markdown which are read."`
	Output string `name:"output" short:"o" help:"File to write the content to, or standard output, when it's omitted, or -." type:"path"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run lists the workspaces
func (cmd *WorkspaceListCmd) Run(ctx server.Cmd) error {
	width, debug := ctx.IsTerm(), ctx.IsDebug()
	return withClient(ctx, "llm.WorkspaceListCmd", func(ctx context.Context, client *httpclient.Client) error {
		list, err := client.ListWorkspaces(ctx)
		if err != nil {
			return err
		} else if debug {
			fmt.Println(list)
			return nil
		}
		_, err = tui.TableFor[schema.Workspace](tui.SetWidth(width)).Write(os.Stdout, list.Body...)
		return err
	})
}

// Run prints the workspace
func (cmd *WorkspaceGetCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.WorkspaceGetCmd", func(ctx context.Context, client *httpclient.Client) error {
		ws, err := client.GetWorkspace(ctx, cmd.Name)
		if err != nil {
			return err
		}
		fmt.Println(ws)
		return nil
	})
}

// Run lists a page of the workspace's folders and resources, and says the
// cursor of the next, when there is one
func (cmd *WorkspaceResourceCmd) Run(ctx server.Cmd) error {
	width, debug := ctx.IsTerm(), ctx.IsDebug()
	return withClient(ctx, "llm.WorkspaceResourceCmd", func(ctx context.Context, client *httpclient.Client) error {
		list, err := client.ListResources(ctx, cmd.Name, cmd.ListRequest)
		if err != nil {
			return err
		} else if debug {
			fmt.Println(list)
			return nil
		}
		if _, err := tui.TableFor[schema.ResourceMeta](tui.SetWidth(width)).Write(os.Stdout, list.Body...); err != nil {
			return err
		}
		if list.Next != "" {
			fmt.Fprintf(os.Stderr, "More with --cursor=%q\n", list.Next)
		}
		return nil
	})
}

// Run reads the resource, by its URI, whose workspace is in it, and writes
// its content to the file, or standard output
func (cmd *WorkspaceReadCmd) Run(ctx server.Cmd) (err error) {
	name, _, ok := schema.ParseURI(cmd.URI)
	if !ok {
		return fmt.Errorf("%q isn't a workspace's URI, resource://<workspace>/<uri>, as it's listed", cmd.URI)
	}
	var w io.Writer = os.Stdout
	if cmd.Output != "" && cmd.Output != "-" {
		// The named error, so the deferred function sees the download's
		// error, and its close's is returned
		var f *os.File
		if f, err = os.Create(cmd.Output); err != nil {
			return err
		}
		defer func() {
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				_ = os.Remove(cmd.Output)
			}
		}()
		w = f
	}
	req := schema.ReadRequest{URI: cmd.URI, Format: cmd.Format, Offset: cmd.Offset, Limit: cmd.Limit}
	return withClient(ctx, "llm.WorkspaceReadCmd", func(ctx context.Context, client *httpclient.Client) error {
		_, err := client.ReadResource(ctx, name, req, w)
		return err
	})
}
