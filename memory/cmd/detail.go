package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/memory/httpclient"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// DetailCommands are the commands which get, set and forget the memory's
// details
type DetailCommands struct {
	Details      DetailsCmd      `cmd:"" name:"details" help:"List the details of you, and the assistant, which every session knows." group:"MEMORY"`
	DetailSet    DetailSetCmd    `cmd:"" name:"detail-set" help:"Set a detail, or decline to give it with --declined." group:"MEMORY"`
	DetailForget DetailForgetCmd `cmd:"" name:"detail-forget" help:"Forget a detail, so it isn't known." group:"MEMORY"`
}

type DetailsCmd struct{}

type DetailSetCmd struct {
	Field    string   `arg:"" name:"field" enum:"name,assistant,location,timezone,language" help:"The detail: name, assistant, location, timezone or language."`
	Value    []string `arg:"" name:"value" optional:"" help:"Its value, which can be more than one argument, such as a time zone, like Europe/Berlin."`
	Declined bool     `name:"declined" help:"Decline to give it, so it isn't in the system prompt, or asked for."`
}

type DetailForgetCmd struct {
	Field string `arg:"" name:"field" enum:"name,assistant,location,timezone,language" help:"The detail: name, assistant, location, timezone or language."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the details
func (cmd *DetailsCmd) Run(ctx server.Cmd) error {
	return withDetails(ctx, "memory.DetailsCmd", func(ctx context.Context, client *httpclient.Client) (*schema.Details, error) {
		return client.GetDetails(ctx)
	})
}

// Run sets the detail, and prints the details
func (cmd *DetailSetCmd) Run(ctx server.Cmd) error {
	field, err := schema.ParseField(cmd.Field)
	if err != nil {
		return err
	}
	detail := schema.Detail{Value: strings.TrimSpace(strings.Join(cmd.Value, " ")), Declined: cmd.Declined}
	if detail.Value == "" && !detail.Declined {
		return fmt.Errorf("missing value, or --declined")
	} else if detail.Value != "" && detail.Declined {
		return fmt.Errorf("a declined detail has no value")
	}
	return withDetails(ctx, "memory.DetailSetCmd", func(ctx context.Context, client *httpclient.Client) (*schema.Details, error) {
		return client.SetDetail(ctx, field, detail)
	})
}

// Run forgets the detail, and prints the details
func (cmd *DetailForgetCmd) Run(ctx server.Cmd) error {
	field, err := schema.ParseField(cmd.Field)
	if err != nil {
		return err
	}
	return withDetails(ctx, "memory.DetailForgetCmd", func(ctx context.Context, client *httpclient.Client) (*schema.Details, error) {
		return client.ForgetDetail(ctx, field)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// withDetails calls the function with a client of the memory's API, and
// prints the details it returns, as a table, or, with debugging, as they
// are
func withDetails(ctx server.Cmd, span string, fn func(context.Context, *httpclient.Client) (*schema.Details, error)) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, span, func(ctx context.Context, client *httpclient.Client) error {
		details, err := fn(ctx, client)
		if err != nil {
			return err
		} else if debug {
			fmt.Println(details)
			return nil
		}
		_, err = tui.TableFor[schema.DetailRow](tui.SetWidth(width)).Write(os.Stdout, details.Rows()...)
		return err
	})
}
