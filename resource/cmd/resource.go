package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/resource/httpclient"
	schema "github.com/mutablelogic/go-llm/resource/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ResourceCommands are the commands which list, upload, get, download and
// delete the resources
type ResourceCommands struct {
	Resources      ResourcesCmd      `cmd:"" name:"resources" help:"List the resources on their own, or a session's, the most recently used first." group:"RESOURCE"`
	Resource       ResourceCmd       `cmd:"" name:"resource" help:"Print a resource." group:"RESOURCE"`
	ResourceCreate ResourceCreateCmd `cmd:"" name:"resource-create" help:"Upload a file, or standard input, as a resource, on its own, or in a session." group:"RESOURCE"`
	ResourceGet    ResourceGetCmd    `cmd:"" name:"resource-get" help:"Download a resource's content, to a file, or standard output." group:"RESOURCE"`
	ResourceDelete ResourceDeleteCmd `cmd:"" name:"resource-delete" help:"Delete a resource." group:"RESOURCE"`
}

type ResourcesCmd struct {
	Session string `name:"session" help:"Identifier of the session whose resources are listed, or none, for those on their own."`
	Offset  uint   `name:"offset" help:"How many resources to skip."`
	Limit   *uint  `name:"limit" help:"Most resources to list."`
}

type ResourceCmd struct {
	Hash    string `arg:"" name:"hash" help:"Hash of the resource."`
	Session string `name:"session" help:"Identifier of the session the resource is in, or none, for one on its own."`
}

type ResourceCreateCmd struct {
	File        string `arg:"" name:"file" optional:"" help:"File to upload, or standard input, when it's omitted, or -." type:"path"`
	Session     string `name:"session" help:"Identifier of the session the resource is in, which it's deleted with, or none, for one on its own, which is deleted once it isn't used."`
	Name        string `name:"name" help:"Name of the resource, which is the file's, when it's omitted."`
	Description string `name:"description" help:"Description of the resource."`
	Type        string `name:"type" help:"MIME type of the resource, which is detected, when it's omitted."`
}

type ResourceGetCmd struct {
	Hash    string `arg:"" name:"hash" help:"Hash of the resource."`
	Session string `name:"session" help:"Identifier of the session the resource is in, or none, for one on its own."`
	Output  string `name:"output" short:"o" help:"File to write the content to, or standard output, when it's omitted, or -." type:"path"`
}

type ResourceDeleteCmd struct {
	Hash    string `arg:"" name:"hash" help:"Hash of the resource."`
	Session string `name:"session" help:"Identifier of the session the resource is in, or none, for one on its own."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the resources, as a table, and how many there are
func (cmd *ResourcesCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "resource.ResourcesCmd", func(ctx context.Context, client *httpclient.Client) error {
		list, err := client.ListResources(ctx, schema.ResourceListRequest{Session: cmd.Session, Offset: cmd.Offset, Limit: cmd.Limit})
		if err != nil {
			return err
		} else if debug {
			fmt.Println(list)
			return nil
		}
		if _, err := tui.TableFor[schema.Resource](tui.SetWidth(width)).Write(os.Stdout, list.Body...); err != nil {
			return err
		}
		var limit *uint64
		if cmd.Limit != nil {
			limit = new(uint64)
			*limit = uint64(*cmd.Limit)
		}
		_, err = tui.TableSummary("resources", list.Count, uint(len(list.Body)), uint64(cmd.Offset), limit).Write(os.Stdout)
		return err
	})
}

// Run prints the resource
func (cmd *ResourceCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "resource.ResourceCmd", func(ctx context.Context, client *httpclient.Client) error {
		resource, err := client.GetResource(ctx, cmd.Hash, schema.ResourceRequest{Session: cmd.Session})
		if err != nil {
			return err
		}
		return printResource(resource, debug)
	})
}

// Run uploads the file, or standard input, and prints the resource
func (cmd *ResourceCreateCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	var content io.Reader = os.Stdin
	file := ""
	if cmd.File != "" && cmd.File != "-" {
		f, err := os.Open(cmd.File)
		if err != nil {
			return err
		}
		defer f.Close()
		content, file = f, filepath.Base(cmd.File)
	}
	req := schema.ResourceCreateRequest{Session: cmd.Session, Name: cmd.Name, Description: cmd.Description, Type: cmd.Type}
	return withClient(ctx, "resource.ResourceCreateCmd", func(ctx context.Context, client *httpclient.Client) error {
		resource, err := client.CreateResource(ctx, req, file, content)
		if err != nil {
			return err
		}
		return printResource(resource, debug)
	})
}

// Run writes the resource's content to the file, or standard output. A
// file which can't be written whole is removed.
func (cmd *ResourceGetCmd) Run(ctx server.Cmd) (err error) {
	var w io.Writer = os.Stdout
	if cmd.Output != "" && cmd.Output != "-" {
		// The named error, rather than one of this block, so the deferred
		// function sees the download's error, and its close's is returned
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
	return withClient(ctx, "resource.ResourceGetCmd", func(ctx context.Context, client *httpclient.Client) error {
		_, err := client.ReadResource(ctx, cmd.Hash, schema.ResourceRequest{Session: cmd.Session}, w)
		return err
	})
}

// Run deletes the resource, and prints it, as it was
func (cmd *ResourceDeleteCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "resource.ResourceDeleteCmd", func(ctx context.Context, client *httpclient.Client) error {
		resource, err := client.DeleteResource(ctx, cmd.Hash, schema.ResourceRequest{Session: cmd.Session})
		if err != nil {
			return err
		}
		return printResource(resource, debug)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// printResource prints the resource: its URI, name, type and size, and its
// session, and description, when it has them, or all of it, as JSON, when
// debugging
func printResource(resource *schema.Resource, debug bool) error {
	if debug {
		fmt.Println(resource)
		return nil
	}
	fmt.Println(resource.URI())
	if resource.Name != "" {
		fmt.Printf("Name: %s\n", resource.Name)
	}
	fmt.Printf("Type: %s, %d bytes\n", resource.Type, resource.Size)
	if resource.Session != nil {
		fmt.Printf("Session: %s\n", resource.Session)
	}
	if resource.Description != "" {
		fmt.Println(resource.Description)
	}
	return nil
}
