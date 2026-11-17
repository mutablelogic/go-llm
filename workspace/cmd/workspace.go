package cmd

import (
	"context"
	"fmt"
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
	Resources  WorkspaceResourceCmd `cmd:"" name:"workspace-resources" help:"List a workspace's folders and resources, in a folder, or under it, with --recursive, a page at a time." group:"WORKSPACES"`
}

type WorkspaceListCmd struct{}

type WorkspaceGetCmd struct {
	Name string `arg:"" name:"name" help:"Name of the workspace, which is its namespace's."`
}

type WorkspaceResourceCmd struct {
	Name string `arg:"" name:"name" help:"Name of the workspace, which is its namespace's."`
	schema.ListRequest
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
