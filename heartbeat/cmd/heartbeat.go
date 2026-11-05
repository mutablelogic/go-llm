package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"uuid"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/heartbeat/httpclient"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// HeartbeatCommands are the commands which list, get, create, change and
// delete the heartbeats
type HeartbeatCommands struct {
	Heartbeats      HeartbeatsCmd      `cmd:"" name:"heartbeats" help:"List the heartbeats, which fire soonest first." group:"HEARTBEAT"`
	Heartbeat       HeartbeatCmd       `cmd:"" name:"heartbeat" help:"Print a heartbeat." group:"HEARTBEAT"`
	HeartbeatCreate HeartbeatCreateCmd `cmd:"" name:"heartbeat-create" help:"Create a heartbeat, a prompt for a session, from the arguments, or standard input, which fires at a time, or on a schedule." group:"HEARTBEAT"`
	HeartbeatUpdate HeartbeatUpdateCmd `cmd:"" name:"heartbeat-update" help:"Change a heartbeat's title, prompt, time, schedule or time zone." group:"HEARTBEAT"`
	HeartbeatDelete HeartbeatDeleteCmd `cmd:"" name:"heartbeat-delete" help:"Delete a heartbeat." group:"HEARTBEAT"`
}

type HeartbeatsCmd struct {
	Session string  `name:"session" help:"Identifier of the session whose heartbeats are listed."`
	Offset  uint64  `name:"offset" help:"How many heartbeats to skip."`
	Limit   *uint64 `name:"limit" help:"Most heartbeats to list."`
}

type HeartbeatCmd struct {
	ID string `arg:"" name:"id" help:"Identifier of the heartbeat."`
}

type HeartbeatCreateCmd struct {
	Prompt   []string `arg:"" name:"prompt" optional:"" help:"The instruction for the model, when it fires, which can be more than one argument, or is read from standard input."`
	Session  string   `name:"session" required:"" help:"Identifier of the session which the prompt is for."`
	Title    string   `name:"title" short:"t" required:"" help:"What it's for, in a few words."`
	Schedule string   `name:"schedule" help:"When it fires, again and again, as a cron expression, such as '0 8 * * 1-5'."`
	At       string   `name:"at" help:"When it fires, once, such as 2026-11-06T15:00, in local time, or with an offset."`
	Timezone string   `name:"timezone" help:"Time zone of the schedule, such as Europe/Berlin, or none, for the person's."`
}

type HeartbeatUpdateCmd struct {
	ID       string  `arg:"" name:"id" help:"Identifier of the heartbeat."`
	Title    *string `name:"title" short:"t" help:"What it's for, in a few words."`
	Prompt   *string `name:"prompt" help:"The instruction for the model, when it fires."`
	Schedule *string `name:"schedule" help:"When it fires, again and again, as a cron expression, which replaces its time."`
	At       string  `name:"at" help:"When it fires, once, which replaces its schedule, in local time, or with an offset."`
	Timezone *string `name:"timezone" help:"Time zone of its schedule, or empty, for the person's."`
}

type HeartbeatDeleteCmd struct {
	ID string `arg:"" name:"id" help:"Identifier of the heartbeat."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the heartbeats, as a table, and how many there are
func (cmd *HeartbeatsCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "heartbeat.HeartbeatsCmd", func(ctx context.Context, client *httpclient.Client) error {
		list, err := client.ListHeartbeats(ctx, schema.HeartbeatListRequest{Session: cmd.Session, Offset: cmd.Offset, Limit: cmd.Limit})
		if err != nil {
			return err
		} else if debug {
			fmt.Println(list)
			return nil
		}
		if _, err := tui.TableFor[schema.Heartbeat](tui.SetWidth(width)).Write(os.Stdout, list.Body...); err != nil {
			return err
		}
		_, err = tui.TableSummary("heartbeats", list.Count, uint(len(list.Body)), cmd.Offset, cmd.Limit).Write(os.Stdout)
		return err
	})
}

// Run prints the heartbeat
func (cmd *HeartbeatCmd) Run(ctx server.Cmd) error {
	id, err := parseID(cmd.ID)
	if err != nil {
		return err
	}
	debug := ctx.IsDebug()
	return withClient(ctx, "heartbeat.HeartbeatCmd", func(ctx context.Context, client *httpclient.Client) error {
		heartbeat, err := client.GetHeartbeat(ctx, id)
		if err != nil {
			return err
		}
		return printHeartbeat(heartbeat, debug)
	})
}

// Run creates the heartbeat, and prints it
func (cmd *HeartbeatCreateCmd) Run(ctx server.Cmd) error {
	session, err := uuid.Parse(cmd.Session)
	if err != nil {
		return fmt.Errorf("invalid session %q", cmd.Session)
	}
	at, err := parseTime(cmd.At)
	if err != nil {
		return err
	}
	prompt := strings.Join(cmd.Prompt, " ")
	if strings.TrimSpace(prompt) == "" {
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return fmt.Errorf("missing the prompt, as arguments, or on standard input")
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		prompt = string(data)
	}
	debug := ctx.IsDebug()
	return withClient(ctx, "heartbeat.HeartbeatCreateCmd", func(ctx context.Context, client *httpclient.Client) error {
		heartbeat, err := client.CreateHeartbeat(ctx, schema.HeartbeatMeta{Title: cmd.Title, Prompt: prompt, Session: session, Schedule: cmd.Schedule, At: at, Timezone: cmd.Timezone})
		if err != nil {
			return err
		}
		return printHeartbeat(heartbeat, debug)
	})
}

// Run changes the heartbeat, and prints it
func (cmd *HeartbeatUpdateCmd) Run(ctx server.Cmd) error {
	id, err := parseID(cmd.ID)
	if err != nil {
		return err
	}
	at, err := parseTime(cmd.At)
	if err != nil {
		return err
	}
	debug := ctx.IsDebug()
	return withClient(ctx, "heartbeat.HeartbeatUpdateCmd", func(ctx context.Context, client *httpclient.Client) error {
		heartbeat, err := client.UpdateHeartbeat(ctx, id, schema.HeartbeatRequest{Title: cmd.Title, Prompt: cmd.Prompt, Schedule: cmd.Schedule, At: at, Timezone: cmd.Timezone})
		if err != nil {
			return err
		}
		return printHeartbeat(heartbeat, debug)
	})
}

// Run deletes the heartbeat, and prints it, as it was
func (cmd *HeartbeatDeleteCmd) Run(ctx server.Cmd) error {
	id, err := parseID(cmd.ID)
	if err != nil {
		return err
	}
	debug := ctx.IsDebug()
	return withClient(ctx, "heartbeat.HeartbeatDeleteCmd", func(ctx context.Context, client *httpclient.Client) error {
		heartbeat, err := client.DeleteHeartbeat(ctx, id)
		if err != nil {
			return err
		}
		return printHeartbeat(heartbeat, debug)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// parseID returns the heartbeat's identifier
func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil(), fmt.Errorf("invalid heartbeat id %q", s)
	}
	return id, nil
}

// parseTime returns the time, with its offset, or in local time, without
// one, or nil, when there's none
func parseTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return types.Ptr(t), nil
		}
	}
	return nil, fmt.Errorf("invalid time %q, which must be such as 2026-11-06T15:00, or 2026-11-06T15:00:00+01:00", s)
}

// printHeartbeat prints the heartbeat's title, when it fires, when it fires
// next, and its prompt, or, with debugging, the heartbeat as it is
func printHeartbeat(heartbeat *schema.Heartbeat, debug bool) error {
	if debug {
		fmt.Println(heartbeat)
		return nil
	}
	fmt.Printf("%s: %s\n", heartbeat.ID, heartbeat.Title)
	fmt.Printf("When %s, for session %s\n", heartbeat.When(), heartbeat.Session)
	if heartbeat.Fired != nil {
		fmt.Printf("Fired %s\n", heartbeat.Fired.Local().Format(time.DateTime))
	}
	if heartbeat.Next != nil {
		fmt.Printf("Next %s\n", heartbeat.Next.Local().Format(time.DateTime))
	}
	fmt.Printf("\n%s\n", heartbeat.Prompt)
	return nil
}
