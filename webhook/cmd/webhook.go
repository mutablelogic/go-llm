package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"uuid"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/webhook/httpclient"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// WebhookCommands are the commands which list, get, create, change and
// delete the webhooks
type WebhookCommands struct {
	Webhooks      WebhooksCmd      `cmd:"" name:"webhooks" help:"List the webhooks, newest first." group:"WEBHOOK"`
	Webhook       WebhookCmd       `cmd:"" name:"webhook" help:"Print a webhook." group:"WEBHOOK"`
	WebhookCreate WebhookCreateCmd `cmd:"" name:"webhook-create" help:"Create a webhook, for a session, and print its URL, which isn't printed again." group:"WEBHOOK"`
	WebhookUpdate WebhookUpdateCmd `cmd:"" name:"webhook-update" help:"Change a webhook's description, or its secret." group:"WEBHOOK"`
	WebhookDelete WebhookDeleteCmd `cmd:"" name:"webhook-delete" help:"Delete a webhook, and its calls, so its URL stops working." group:"WEBHOOK"`
}

type WebhooksCmd struct {
	Session string  `name:"session" help:"Identifier of the session whose webhooks are listed."`
	Offset  uint64  `name:"offset" help:"How many webhooks to skip."`
	Limit   *uint64 `name:"limit" help:"Most webhooks to list."`
}

type WebhookCmd struct {
	ID uint64 `arg:"" name:"id" help:"Serial number of the webhook."`
}

type WebhookCreateCmd struct {
	Description []string `arg:"" name:"description" help:"What the webhook is for, which can be more than one argument."`
	Session     string   `name:"session" required:"" help:"Identifier of the session which its calls are for."`
	Type        string   `name:"type" help:"Protocol of the service which calls it: generic, github, aftership or pidgeon." default:"generic"`
	Secret      string   `name:"secret" env:"WEBHOOK_SECRET" help:"Secret which the service signs its calls with, when it gives one."`
}

type WebhookUpdateCmd struct {
	ID          uint64  `arg:"" name:"id" help:"Serial number of the webhook."`
	Description *string `name:"description" help:"What the webhook is for."`
	Secret      *string `name:"secret" help:"Secret which the service signs its calls with, or empty, to remove it."`
}

type WebhookDeleteCmd struct {
	ID uint64 `arg:"" name:"id" help:"Serial number of the webhook."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the webhooks, as a table, and how many there are
func (cmd *WebhooksCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "webhook.WebhooksCmd", func(ctx context.Context, client *httpclient.Client) error {
		list, err := client.ListWebhooks(ctx, schema.WebhookListRequest{Session: cmd.Session, Offset: cmd.Offset, Limit: cmd.Limit})
		if err != nil {
			return err
		} else if debug {
			fmt.Println(list)
			return nil
		}
		if _, err := tui.TableFor[schema.Webhook](tui.SetWidth(width)).Write(os.Stdout, list.Body...); err != nil {
			return err
		}
		_, err = tui.TableSummary("webhooks", list.Count, uint(len(list.Body)), cmd.Offset, cmd.Limit).Write(os.Stdout)
		return err
	})
}

// Run prints the webhook
func (cmd *WebhookCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "webhook.WebhookCmd", func(ctx context.Context, client *httpclient.Client) error {
		webhook, err := client.GetWebhook(ctx, cmd.ID)
		if err != nil {
			return err
		}
		return printWebhook(*webhook, debug)
	})
}

// Run creates the webhook, and prints it, with its URL, or its token, when
// the server has no public endpoint
func (cmd *WebhookCreateCmd) Run(ctx server.Cmd) error {
	session, err := uuid.Parse(strings.TrimSpace(cmd.Session))
	if err != nil {
		return fmt.Errorf("invalid session %q", cmd.Session)
	}
	debug := ctx.IsDebug()
	return withClient(ctx, "webhook.WebhookCreateCmd", func(ctx context.Context, client *httpclient.Client) error {
		webhook, err := client.CreateWebhook(ctx, schema.WebhookMeta{Session: session, Description: strings.Join(cmd.Description, " "), Type: cmd.Type, Secret: cmd.Secret})
		if err != nil {
			return err
		} else if debug {
			fmt.Println(webhook)
			return nil
		}
		if err := printWebhook(webhook.Webhook, false); err != nil {
			return err
		}
		if webhook.URL != "" {
			fmt.Printf("\nURL, which isn't printed again:\n%s\n", webhook.URL)
		} else {
			fmt.Printf("\nToken, which isn't printed again, as the server has no public endpoint, after its /hook/ path:\n%s\n", webhook.Token)
		}
		return nil
	})
}

// Run changes the webhook, and prints it
func (cmd *WebhookUpdateCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "webhook.WebhookUpdateCmd", func(ctx context.Context, client *httpclient.Client) error {
		webhook, err := client.UpdateWebhook(ctx, cmd.ID, schema.WebhookRequest{Description: cmd.Description, Secret: cmd.Secret})
		if err != nil {
			return err
		}
		return printWebhook(*webhook, debug)
	})
}

// Run deletes the webhook, and prints it, as it was
func (cmd *WebhookDeleteCmd) Run(ctx server.Cmd) error {
	debug := ctx.IsDebug()
	return withClient(ctx, "webhook.WebhookDeleteCmd", func(ctx context.Context, client *httpclient.Client) error {
		webhook, err := client.DeleteWebhook(ctx, cmd.ID)
		if err != nil {
			return err
		}
		return printWebhook(*webhook, debug)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// printWebhook prints the webhook's serial number, description, type,
// session, whether it has a secret, and when it was created, or, with
// debugging, the webhook as it is
func printWebhook(webhook schema.Webhook, debug bool) error {
	if debug {
		fmt.Println(webhook)
		return nil
	}
	description := webhook.Description
	if description == "" {
		description = "(no description)"
	}
	secret := "without a secret"
	if webhook.HasSecret {
		secret = "with a secret"
	}
	fmt.Printf("%s: %s\n", strconv.FormatUint(webhook.ID, 10), description)
	fmt.Printf("%s, %s, for session %s\n", webhook.Type, secret, webhook.Session)
	fmt.Printf("Created %s\n", webhook.Created.Local().Format(time.DateTime))
	return nil
}
