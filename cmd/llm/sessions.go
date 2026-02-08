package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SessionCommands struct {
	ListSessions   ListSessionsCommand   `cmd:"" name:"sessions" help:"List stored chat sessions." group:"SESSION"`
	ShowSession    ShowSessionCommand    `cmd:"" name:"session" help:"Show session details." group:"SESSION"`
	DeleteSession  DeleteSessionCommand  `cmd:"" name:"delete-session" help:"Delete a stored session." group:"SESSION"`
	DeleteSessions DeleteSessionsCommand `cmd:"" name:"delete-sessions" help:"Delete all stored sessions." group:"SESSION"`
}

type ListSessionsCommand struct{}

type ShowSessionCommand struct {
	ID string `arg:"" name:"id" help:"Session ID"`
}

type DeleteSessionCommand struct {
	ID string `arg:"" name:"id" help:"Session ID to delete"`
}

type DeleteSessionsCommand struct{}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListSessionsCommand) Run(ctx *Globals) error {
	store, err := ctx.Store()
	if err != nil {
		return err
	}

	sessions, err := store.List(ctx.ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tMODEL\tMESSAGES\tTOKENS\tMODIFIED")
	for _, s := range sessions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
			s.ID,
			s.Name,
			s.Model.Name,
			len(s.Messages),
			s.Tokens(),
			humanTime(s.Modified),
		)
	}
	w.Flush()

	return nil
}

func (cmd *ShowSessionCommand) Run(ctx *Globals) error {
	store, err := ctx.Store()
	if err != nil {
		return err
	}

	s, err := store.Get(ctx.ctx, cmd.ID)
	if err != nil {
		return fmt.Errorf("session %q: %w", cmd.ID, err)
	}

	fmt.Printf("ID:       %s\n", s.ID)
	fmt.Printf("Name:     %s\n", s.Name)
	fmt.Printf("Model:    %s\n", s.Model.Name)
	fmt.Printf("Messages: %d\n", len(s.Messages))
	fmt.Printf("Tokens:   %d\n", s.Tokens())
	fmt.Printf("Created:  %s\n", s.Created.Format(time.RFC3339))
	fmt.Printf("Modified: %s\n", s.Modified.Format(time.RFC3339))

	// Print messages
	if len(s.Messages) > 0 {
		fmt.Println()
		for i, m := range s.Messages {
			fmt.Printf("[%d] %s: %s\n", i+1, m.Role, m.Text())
		}
	}

	return nil
}

func (cmd *DeleteSessionCommand) Run(ctx *Globals) error {
	store, err := ctx.Store()
	if err != nil {
		return err
	}

	if err := store.Delete(ctx.ctx, cmd.ID); err != nil {
		return fmt.Errorf("session %q: %w", cmd.ID, err)
	}

	fmt.Fprintf(os.Stderr, "Deleted session %s\n", cmd.ID)
	return nil
}

func (cmd *DeleteSessionsCommand) Run(ctx *Globals) error {
	store, err := ctx.Store()
	if err != nil {
		return err
	}

	sessions, err := store.List(ctx.ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	for _, s := range sessions {
		if err := store.Delete(ctx.ctx, s.ID); err != nil {
			return fmt.Errorf("session %q: %w", s.ID, err)
		}
	}

	fmt.Fprintf(os.Stderr, "Deleted %d session(s)\n", len(sessions))
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// humanTime returns a human-friendly relative time string.
func humanTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("2006-01-02")
	}
}
