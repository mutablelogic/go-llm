package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CallCommands are the commands which list, and get, the webhooks' calls
type CallCommands struct {
	Calls CallsCmd `cmd:"" name:"calls" help:"List the webhooks' calls, oldest first, which were accepted, or refused, or both." group:"WEBHOOK"`
	Call  CallCmd  `cmd:"" name:"call" help:"Print a call, with its headers, and what it sent." group:"WEBHOOK"`
}

type CallsCmd struct {
	Webhook uint64  `name:"webhook" help:"Serial number of the webhook whose calls are listed."`
	Session string  `name:"session" help:"Identifier of the session whose webhooks' calls are listed."`
	Status  string  `name:"status" enum:",accepted,refused" default:"" help:"Which calls are listed: accepted, refused, or both, when it's not set."`
	After   uint64  `name:"after" help:"Serial number of the call after which they're listed."`
	Limit   *uint64 `name:"limit" help:"Most calls to list."`
}

type CallCmd struct {
	ID uint64 `arg:"" name:"id" help:"Serial number of the call."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the calls, as a table, and how many there are after the call
func (cmd *CallsCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "webhook.CallsCmd", func(ctx context.Context, client *httpclient.Client) error {
		list, err := client.ListCalls(ctx, schema.CallListRequest{Webhook: cmd.Webhook, Session: cmd.Session, Status: cmd.Status, After: cmd.After, Limit: cmd.Limit})
		if err != nil {
			return err
		} else if debug {
			fmt.Println(list)
			return nil
		}
		if _, err := tui.TableFor[schema.Call](tui.SetWidth(width)).Write(os.Stdout, list.Body...); err != nil {
			return err
		}
		_, err = tui.TableSummary("calls", list.Count, uint(len(list.Body)), 0, cmd.Limit).Write(os.Stdout)
		return err
	})
}

// Run prints the call
func (cmd *CallCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "webhook.CallCmd", func(ctx context.Context, client *httpclient.Client) error {
		call, err := client.GetCall(ctx, cmd.ID)
		if err != nil {
			return err
		}
		return printCall(*call, debug)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// printCall prints what the call was: its serial number, its webhook's,
// when it was received, its method, what happened, whether it was accepted,
// or refused, with why, its query, its headers, and what it sent, as JSON,
// or, with debugging, the call as it is
func printCall(call schema.Call, debug bool) error {
	if debug {
		fmt.Println(call)
		return nil
	}
	fmt.Printf("%d: %s of webhook %d, received %s\n", call.ID, call.Method, call.Webhook, call.Received.Local().Format(time.DateTime))
	if call.Event != "" {
		fmt.Printf("Event %s", call.Event)
		if call.Delivery != "" {
			fmt.Printf(", delivery %s", call.Delivery)
		}
		fmt.Println()
	}
	switch {
	case call.Error != "" && call.Verified:
		fmt.Printf("Refused, though verified: %s\n", call.Error)
	case call.Error != "":
		fmt.Printf("Refused: %s\n", call.Error)
	case call.Verified:
		fmt.Println("Accepted, and verified")
	default:
		fmt.Println("Accepted, without being verified")
	}
	if len(call.Query) > 0 {
		fmt.Println("\nQuery:")
		for _, name := range sortedKeys(call.Query) {
			for _, value := range call.Query[name] {
				fmt.Printf("  %s=%s\n", name, value)
			}
		}
	}
	if len(call.Headers) > 0 {
		fmt.Println("\nHeaders:")
		for _, name := range sortedKeys(call.Headers) {
			fmt.Printf("  %s: %s\n", name, call.Headers[name])
		}
	}
	switch {
	case len(call.Values) > 0:
		var indented bytes.Buffer
		if err := json.Indent(&indented, call.Values, "", "  "); err != nil {
			indented.Reset()
			indented.Write(call.Values)
		}
		fmt.Printf("\n%s\n", indented.String())
	case len(call.Body) > 0:
		fmt.Printf("\n(%d bytes, which aren't text)\n", len(call.Body))
	}
	return nil
}

// sortedKeys returns the map's keys, in order
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
