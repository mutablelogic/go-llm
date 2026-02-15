package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	ansiDim    = "\033[2m"  // Dim text for thinking
	ansiReset  = "\033[0m"  // Reset formatting
	ansiCyan   = "\033[36m" // Cyan for thinking label
	ansiYellow = "\033[33m" // Yellow for tool calls
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type MessageCommands struct {
	Ask  AskCommand  `cmd:"" name:"ask" help:"Send a message to a model." group:"MESSAGE"`
	Chat ChatCommand `cmd:"" name:"chat" help:"Chat with a model in a session." group:"MESSAGE"`
}

type AskCommand struct {
	Model     string   `arg:"" name:"model" help:"Model name"`
	Text      string   `arg:"" name:"text" help:"Message text to send"`
	File      []string `name:"file" help:"File path(s) to attach (can be used multiple times)" type:"existingfile"`
	Streaming bool     `name:"streaming" help:"Enable streaming output" default:"true" negatable:""`
	Thinking  bool     `name:"thinking" help:"Enable extended thinking/reasoning"`
	JSON      string   `name:"json" help:"Path to a JSON schema file to constrain output" type:"existingfile"`
}

type ChatCommand struct {
	Model     string `name:"model" short:"m" help:"Model name (required when creating a new session)"`
	Session   string `name:"session" short:"s" help:"Session ID to resume (omit to continue the most recent session)"`
	Streaming bool   `name:"streaming" help:"Enable streaming output" default:"true" negatable:""`
	Thinking  bool   `name:"thinking" help:"Enable extended thinking/reasoning"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *AskCommand) Run(ctx *Globals) (err error) {
	// Get agent
	a, err := ctx.Agent()
	if err != nil {
		return err
	}

	// Get the model
	model, err := a.GetModel(ctx.ctx, cmd.Model)
	if err != nil {
		return fmt.Errorf("failed to get model %q: %w", cmd.Model, err)
	}

	// Build options for message
	var opts []opt.Opt

	// Add files if provided
	for _, filePath := range cmd.File {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %q: %w", filePath, err)
		}
		defer file.Close()

		opts = append(opts, schema.WithAttachment(file))
	}

	// Create message
	message, err := schema.NewMessage(schema.RoleUser, cmd.Text, opts...)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// Build generation options
	var genopts []opt.Opt
	if cmd.Streaming {
		genopts = append(genopts, opt.WithStream(streamCallback()))
	}
	if cmd.Thinking {
		genopts = append(genopts, agent.WithThinking())
	}
	if cmd.JSON != "" {
		s, err := loadJSONSchema(cmd.JSON)
		if err != nil {
			return err
		}
		genopts = append(genopts, agent.WithJSONOutput(s))
	}

	// Send the message
	response, err := a.WithoutSession(ctx.ctx, *model, message, genopts...)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Print the response (newline after streaming, or full text if not streaming)
	if cmd.Streaming {
		fmt.Print(ansiReset)
		fmt.Println()
	} else {
		fmt.Println(response.Text())
	}

	return nil
}

func (cmd *ChatCommand) Run(ctx *Globals) (err error) {
	// Get agent
	a, err := ctx.Agent()
	if err != nil {
		return err
	}

	// Get session store
	store, err := ctx.Store()
	if err != nil {
		return err
	}

	// Resolve session: explicit ID > most recent > create new
	var sess *schema.Session
	switch {
	case cmd.Session != "":
		// Resume a specific session by ID
		sess, err = store.Get(ctx.ctx, cmd.Session)
		if err != nil {
			return fmt.Errorf("failed to load session %q: %w", cmd.Session, err)
		}
		fmt.Fprintf(os.Stderr, "Resumed session %s (%s, %d messages)\n", sess.ID, sess.Name, len(sess.Messages))
	default:
		// Try to continue the most recent session
		recent, err := store.List(ctx.ctx, schema.ListSessionRequest{Limit: 1})
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}
		if len(recent.Body) > 0 {
			sess = recent.Body[0]
			fmt.Fprintf(os.Stderr, "Resumed session %s (%s, %d messages)\n", sess.ID, sess.Name, len(sess.Messages))
		} else {
			// No sessions exist — create a new one (model is required)
			if cmd.Model == "" {
				return fmt.Errorf("no existing sessions; --model is required to start a new session")
			}
			sess, err = store.Create(ctx.ctx, schema.SessionMeta{Name: cmd.Model, Model: cmd.Model})
			if err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
			fmt.Fprintf(os.Stderr, "New session %s\n", sess.ID)
		}
	}

	// If --model is provided and differs from the session model, switch it
	if cmd.Model != "" && cmd.Model != sess.Model {
		sess.Model = cmd.Model
		fmt.Fprintf(os.Stderr, "Switched model to %s\n", cmd.Model)
	}

	// Resolve the model for generation
	model, err := a.GetModel(ctx.ctx, sess.Model)
	if err != nil {
		return fmt.Errorf("failed to get model %q: %w", sess.Model, err)
	}

	// Build generation options
	var genopts []opt.Opt
	if cmd.Streaming {
		genopts = append(genopts, opt.WithStream(streamCallback()))
	}
	if cmd.Thinking {
		genopts = append(genopts, agent.WithThinking())
	}
	// Attach toolkit if any tools are available
	toolkit, err := ctx.Toolkit()
	if err != nil {
		return err
	}
	if len(toolkit.Tools()) > 0 {
		genopts = append(genopts, agent.WithToolkit(toolkit))
		genopts = append(genopts, agent.WithSystemPrompt(
			"You have access to tools. Always call a tool when you can rather than asking the user "+
				"for more information. Make reasonable assumptions for any missing parameters "+
				"(e.g. use a default location, today's date, general category). "+
				"After receiving tool results, always summarize them in a helpful response.",
		))
	}

	// Get the underlying message session
	msgSession := sess.Conversation()

	// Interactive loop: read from stdin until CTRL+C or EOF
	scanner := bufio.NewScanner(os.Stdin)
	for {
		// Print prompt with model name
		fmt.Printf("\n%s> ", sess.Model)

		// Read input line
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		// Check if context was cancelled
		if ctx.ctx.Err() != nil {
			break
		}

		// Create message
		message, err := schema.NewMessage(schema.RoleUser, text)
		if err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}

		// Send the message within the session
		response, err := a.WithSession(ctx.ctx, *model, msgSession, message, genopts...)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		// Print the response
		if cmd.Streaming {
			fmt.Print(ansiReset)
			fmt.Println()
		} else {
			fmt.Println(response.Text())
		}

		// Persist session to disk after each exchange
		sess.Modified = time.Now()
		if err := store.Write(sess); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("input error: %w", err)
	}

	fmt.Println()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// streamCallback returns a StreamFn that prints assistant text normally
// and thinking text in dim gray.
func streamCallback() opt.StreamFn {
	lastRole := ""
	return func(role, text string) {
		if role != lastRole {
			switch role {
			case "thinking":
				fmt.Print(ansiDim + ansiCyan)
			case "tool":
				fmt.Print(ansiReset + ansiDim + ansiYellow)
			default:
				fmt.Print(ansiReset)
				if lastRole == "thinking" || lastRole == "tool" {
					fmt.Println() // newline between thinking/tool and assistant
				}
			}
			lastRole = role
		}
		fmt.Print(text)
	}
}

// loadJSONSchema reads a JSON schema file and returns the parsed schema.
func loadJSONSchema(path string) (*jsonschema.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON schema file %q: %w", path, err)
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to parse JSON schema file %q: %w", path, err)
	}
	return &s, nil
}
