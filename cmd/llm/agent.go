package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	uitable "github.com/mutablelogic/go-llm/pkg/ui/table"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type AgentCommands struct {
	ListAgents  ListAgentsCommand  `cmd:"" name:"agents" help:"List agents." group:"AGENT"`
	GetAgent    GetAgentCommand    `cmd:"" name:"agent" help:"Get an agent." group:"AGENT"`
	CreateAgent CreateAgentCommand `cmd:"" name:"create-agent" help:"Create agents from markdown files." group:"AGENT"`
	DeleteAgent DeleteAgentCommand `cmd:"" name:"delete-agent" help:"Delete an agent." group:"AGENT"`
	RunAgent    RunAgentCommand    `cmd:"" name:"run-agent" help:"Create a session from an agent and chat." group:"AGENT"`
}

type ListAgentsCommand struct {
	Limit  *uint  `name:"limit" help:"Maximum number of agents to return" optional:""`
	Offset uint   `name:"offset" help:"Offset for pagination" default:"0"`
	Name   string `name:"name" help:"Filter by agent name" optional:""`
}

type GetAgentCommand struct {
	ID string `arg:"" name:"id" help:"Agent ID or name (use name@version for a specific version)"`
}

type CreateAgentCommand struct {
	Files []string `arg:"" name:"files" help:"Glob pattern(s) for agent markdown files" required:""`
}

type DeleteAgentCommand struct {
	ID string `arg:"" name:"id" help:"Agent ID or name (use name@version for a specific version)"`
}

type RunAgentCommand struct {
	Agent  string `arg:"" name:"agent" help:"Agent ID or name"`
	Parent string `name:"parent" help:"Parent session ID" optional:""`
	Input  string `name:"input" help:"JSON input for the agent template" optional:""`
	Delete bool   `name:"delete" negatable:"" default:"true" help:"Delete the agent session after completion"`
}

///////////////////////////////////////////////////////////////////////////////
// COMMANDS

func (cmd *ListAgentsCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "ListAgentsCommand")
	defer func() { endSpan(err) }()

	// Build options
	opts := []opt.Opt{}
	if cmd.Limit != nil {
		opts = append(opts, httpclient.WithLimit(cmd.Limit))
	}
	if cmd.Offset > 0 {
		opts = append(opts, httpclient.WithOffset(cmd.Offset))
	}
	if cmd.Name != "" {
		opts = append(opts, httpclient.WithName(cmd.Name))
	}

	// List agents
	response, err := client.ListAgents(parent, opts...)
	if err != nil {
		return err
	}

	// Print
	if ctx.Debug {
		fmt.Println(response)
	} else {
		if len(response.Body) > 0 {
			fmt.Println(uitable.Render(schema.AgentTable(response.Body)))
		}
		fmt.Println(TableSummary(len(response.Body), int(response.Offset), int(response.Count)))
	}
	return nil
}

func (cmd *GetAgentCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "GetAgentCommand")
	defer func() { endSpan(err) }()

	// Parse name@version
	name, version, err := parseAgentID(cmd.ID)
	if err != nil {
		return err
	}

	// Get agent
	if version != nil {
		// Use ListAgents with name + version filter
		response, err := client.ListAgents(parent, httpclient.WithName(name), httpclient.WithVersion(*version))
		if err != nil {
			return err
		}
		if len(response.Body) == 0 {
			return fmt.Errorf("agent %q version %d not found", name, *version)
		}
		fmt.Println(response.Body[0])
	} else {
		agent, err := client.GetAgent(parent, name)
		if err != nil {
			return err
		}
		fmt.Println(agent)
	}
	return nil
}

func (cmd *CreateAgentCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "CreateAgentCommand")
	defer func() { endSpan(err) }()

	// Expand globs and parse files
	var agents []schema.AgentMeta
	seen := make(map[string]string) // name -> file path
	for _, pattern := range cmd.Files {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			return fmt.Errorf("no files matched %q", pattern)
		}
		for _, path := range matches {
			meta, err := agent.ReadFile(path)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if prev, ok := seen[meta.Name]; ok {
				return fmt.Errorf("duplicate agent name %q in %s and %s", meta.Name, prev, path)
			}
			seen[meta.Name] = path
			agents = append(agents, meta)
		}
	}

	// Create or update each agent
	var errs []error
	var created, updated int
	for _, meta := range agents {
		result, isNew, err := createOrUpdateAgent(client, parent, meta)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", meta.Name, err))
			continue
		}
		if result != nil {
			fmt.Println(result)
			if isNew {
				created++
			} else {
				updated++
			}
		}
	}
	if created == 0 && updated == 0 && len(errs) == 0 {
		fmt.Println("no changes")
	} else if created > 0 || updated > 0 {
		fmt.Printf("%d created, %d updated\n", created, updated)
	}

	return errors.Join(errs...)
}

func (cmd *DeleteAgentCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "DeleteAgentCommand")
	defer func() { endSpan(err) }()

	// Parse name@version
	name, version, err := parseAgentID(cmd.ID)
	if err != nil {
		return err
	}

	// Delete agent
	if version != nil {
		// Find specific version via ListAgents, then delete by ID
		response, err := client.ListAgents(parent, httpclient.WithName(name), httpclient.WithVersion(*version))
		if err != nil {
			return err
		}
		if len(response.Body) == 0 {
			return fmt.Errorf("agent %q version %d not found", name, *version)
		}
		if err := client.DeleteAgent(parent, response.Body[0].ID); err != nil {
			return err
		}
	} else {
		if err := client.DeleteAgent(parent, name); err != nil {
			return err
		}
	}

	return nil
}

func (cmd *RunAgentCommand) Run(ctx *Globals) (err error) {
	client, err := ctx.Client()
	if err != nil {
		return err
	}

	// OTEL
	parent, endSpan := otel.StartSpan(ctx.tracer, ctx.ctx, "RunAgentCommand")
	defer func() { endSpan(err) }()

	// Build request
	req := schema.CreateAgentSessionRequest{
		Parent: cmd.Parent,
	}
	if req.Parent == "" {
		req.Parent = ctx.defaults.GetString("session")
	}
	if cmd.Input != "" {
		req.Input = json.RawMessage(cmd.Input)
	}

	// Create agent session
	resp, err := client.CreateAgentSession(parent, cmd.Agent, req)
	if err != nil {
		return err
	}

	// Stream the chat response to stdout
	var lastRole string
	chatReq := schema.ChatRequest{
		Session: resp.Session,
		Text:    resp.Text,
		Tools:   resp.Tools,
	}
	chatOpts := []httpclient.ChatOpt{
		httpclient.WithChatStream(func(role, text string) {
			if role != lastRole {
				if lastRole != "" {
					fmt.Println()
				}
				fmt.Print(role + ": ")
				lastRole = role
			}
			fmt.Print(text)
		}),
	}

	chatResp, err := client.Chat(parent, chatReq, chatOpts...)
	if err != nil {
		return err
	}

	// If the response contains structured output (from submit_output),
	// print it. The streaming callback may have been suppressed, so the
	// response body is the authoritative output.
	if chatResp != nil && len(chatResp.Content) > 0 {
		for _, block := range chatResp.Content {
			if block.Text != nil && *block.Text != "" {
				if lastRole != "" {
					// A role was already printed by streaming; start a new line
					fmt.Println()
				}
				fmt.Println(*block.Text)
				lastRole = "" // prevent the trailing newline below
			}
		}
	}
	if lastRole != "" {
		fmt.Println()
	}

	// Delete the agent session unless --no-delete was specified
	if cmd.Delete {
		if err := client.DeleteSession(parent, resp.Session); err != nil {
			return fmt.Errorf("deleting agent session: %w", err)
		}
	}
	return nil
}

// createOrUpdateAgent creates a new agent or updates an existing one by name.
// Returns the result, whether it was newly created (true) or updated (false),
// and any error. Returns nil result and nil error when the agent was not modified.
func createOrUpdateAgent(client *httpclient.Client, ctx context.Context, meta schema.AgentMeta) (*schema.Agent, bool, error) {
	// Check if agent already exists by name
	if _, err := client.GetAgent(ctx, meta.Name); err == nil {
		result, err := client.UpdateAgent(ctx, meta)
		if isNotModified(err) {
			return nil, false, nil
		}
		return result, false, err
	} else if !isNotFound(err) {
		return nil, false, fmt.Errorf("checking agent %q: %w", meta.Name, err)
	}
	result, err := client.CreateAgent(ctx, meta)
	return result, true, err
}

// isNotFound returns true if the error represents an HTTP 404 response.
func isNotFound(err error) bool {
	var httpErr httpresponse.Err
	return errors.As(err, &httpErr) && int(httpErr) == http.StatusNotFound
}

// isNotModified returns true if the error represents an HTTP 304 response.
func isNotModified(err error) bool {
	var httpErr httpresponse.Err
	return err != nil && errors.As(err, &httpErr) && int(httpErr) == http.StatusNotModified
}

// parseAgentID splits an ID string of the form "name@version" into its
// components. If no "@" is present, the entire string is the name and
// version is nil. Returns an error if a version suffix is present but
// not a valid unsigned integer.
func parseAgentID(id string) (string, *uint, error) {
	name, vstr, ok := strings.Cut(id, "@")
	if !ok {
		return id, nil, nil
	}
	v, err := strconv.ParseUint(vstr, 10, 64)
	if err != nil {
		return "", nil, fmt.Errorf("invalid version in %q: %w", id, err)
	}
	u := uint(v)
	return name, &u, nil
}
