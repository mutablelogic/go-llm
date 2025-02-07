package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	telegram "github.com/mutablelogic/go-llm/pkg/ui/telegram"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type Chat2Cmd struct {
	Model string `arg:"" help:"Model name"`
	Token string `env:"TELEGRAM_TOKEN" help:"Telegram token" required:""`
}

type Server struct {
	sync.RWMutex
	*telegram.Client

	// Model and toolkit
	toolkit llm.ToolKit
	model   llm.Model

	// Map of active sessions
	sessions map[string]llm.Context
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewTelegramServer(token string, model llm.Model, toolkit llm.ToolKit, opts ...telegram.Opt) (*Server, error) {
	server := new(Server)
	server.sessions = make(map[string]llm.Context)
	server.model = model
	server.toolkit = toolkit

	// Create a new telegram client
	opts = append(opts, telegram.WithCallback(server.receive))
	if telegram, err := telegram.New(token, opts...); err != nil {
		return nil, err
	} else {
		server.Client = telegram
	}

	// Return success
	return server, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *Chat2Cmd) Run(globals *Globals) error {
	return run(globals, cmd.Model, func(ctx context.Context, model llm.Model) error {
		server, err := NewTelegramServer(cmd.Token, model, globals.toolkit, telegram.WithDebug(globals.Debug))
		if err != nil {
			return err
		}

		log.Printf("Running Telegram bot %q\n", server.Client.Name())

		var result error
		var wg sync.WaitGroup
		wg.Add(2)
		go func(ctx context.Context) {
			defer wg.Done()
			if err := server.Run(ctx); err != nil {
				result = errors.Join(result, err)
			}
		}(ctx)
		go func(ctx context.Context) {
			defer wg.Done()
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					server.Purge()
				}
			}
		}(ctx)

		// Wait for completion
		wg.Wait()

		// Return any errors
		return result
	})
}

// //////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (telegram *Server) Purge() {
	telegram.Lock()
	defer telegram.Unlock()
	for user, session := range telegram.sessions {
		if session.SinceLast() > 10*time.Minute {
			log.Printf("Purging session for %q\n", user)
			delete(telegram.sessions, user)
		}
	}
}

func (telegram *Server) session(user string) llm.Context {
	telegram.Lock()
	defer telegram.Unlock()
	if session, exists := telegram.sessions[user]; exists {
		return session
	}
	session := telegram.model.Context(
		llm.WithToolKit(telegram.toolkit),
		llm.WithSystemPrompt("Please reply to messages in markdown format."),
	)
	telegram.sessions[user] = session
	return session
}

func (telegram *Server) receive(ctx context.Context, msg telegram.Message) error {
	// Get an active session
	session := telegram.session(msg.Sender())

	// Process the message
	text := msg.Text()
	text += "\n\nPlease reply in markdown format."
	if err := session.FromUser(ctx, text); err != nil {
		return err
	}

	// Run tool calls
	for {
		calls := session.ToolCalls(0)
		if len(calls) == 0 {
			break
		}
		if text := session.Text(0); text != "" {
			msg.Reply(ctx, text, false)
		} else {
			msg.Reply(ctx, "_Gathering information_", true)
		}

		results, err := telegram.toolkit.Run(ctx, calls...)
		if err != nil {
			return err
		} else if err := session.FromTool(ctx, results...); err != nil {
			return err
		}
	}

	// Reply with the text
	return msg.Reply(ctx, session.Text(0), true)
}
