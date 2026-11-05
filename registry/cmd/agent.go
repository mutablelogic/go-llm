package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	tui "github.com/mutablelogic/go-server/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AgentCommands are the commands of agents, whose definitions are YAML, as
// a file of an agent's definition is
type AgentCommands struct {
	AgentList   AgentListCmd   `cmd:"" name:"agents" help:"List the agents." group:"LLM TOOLS & AGENTS"`
	AgentGet    AgentGetCmd    `cmd:"" name:"agent" help:"Print an agent's definition, as YAML." group:"LLM TOOLS & AGENTS"`
	AgentCreate AgentCreateCmd `cmd:"" name:"agent-create" help:"Create an agent from a YAML file, or standard input." group:"LLM TOOLS & AGENTS"`
	AgentUpdate AgentUpdateCmd `cmd:"" name:"agent-update" help:"Change an agent's definition. An empty value clears one, and --opt key=null removes an option." group:"LLM TOOLS & AGENTS"`
	AgentDelete AgentDeleteCmd `cmd:"" name:"agent-delete" help:"Delete an agent, and print its definition, as YAML." group:"LLM TOOLS & AGENTS"`
}

// AgentListCmd lists the agents
type AgentListCmd struct {
	Offset uint  `name:"offset" help:"Agents to skip." optional:""`
	Limit  *uint `name:"limit" help:"Most agents to list." optional:""`
}

// AgentGetCmd prints an agent's definition, as YAML
type AgentGetCmd struct {
	Name string `arg:"" name:"name" help:"Name of the agent."`
}

// AgentUpdateCmd changes the definition of an agent, which are given, and
// keeps the rest: an empty value, such as --tools=, clears one, --input=null
// and --schema=null clear the schemas, and an option which is null, such as
// --opt thinking=null, is removed
type AgentUpdateCmd struct {
	Name        string          `arg:"" name:"name" help:"Name of the agent."`
	Title       *string         `name:"title" help:"Title of the agent, for people." optional:""`
	Description *string         `name:"description" help:"What the agent does, for people, and models which call it as a tool." optional:""`
	Input       json.RawMessage `name:"input" help:"JSON schema of the input, which is an object, or null to clear it." optional:""`
	Prompt      *string         `name:"prompt" help:"Go template of the message to the model, executed with the input." optional:""`
	schema.GeneratorMeta
}

// AgentDeleteCmd deletes an agent
type AgentDeleteCmd struct {
	Name string `arg:"" name:"name" help:"Name of the agent."`
}

// AgentCreateCmd creates an agent from its definition, in YAML, which is
// read from a file, or standard input
type AgentCreateCmd struct {
	File string `arg:"" name:"file" optional:"" help:"File of the agent's definition, in YAML, or - for standard input, which it's read from when there's no file."`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints a table of the agents, and a summary, or with debugging, the
// response
func (cmd *AgentListCmd) Run(ctx server.Cmd) error {
	width := ctx.IsTerm()
	debug := ctx.IsDebug()
	return withClient(ctx, "llm.AgentListCmd", func(ctx context.Context, client *httpclient.Client) error {
		agents, err := client.ListAgents(ctx, schema.AgentListRequest{Offset: cmd.Offset, Limit: cmd.Limit})
		if err != nil {
			return err
		} else if debug {
			fmt.Println(agents)
			return nil
		}
		table := tui.TableFor[schema.Agent](tui.SetWidth(width))
		if _, err := table.Write(os.Stdout, agents.Body...); err != nil {
			return err
		}
		summary := tui.TableSummary("agents", agents.Count, uint(len(agents.Body)), uint64(cmd.Offset), nil)
		_, err = summary.Write(os.Stdout)
		return err
	})
}

// Run changes the agent's definition, which is given, and prints it, as YAML
func (cmd *AgentUpdateCmd) Run(ctx server.Cmd) error {
	meta := schema.AgentMeta{
		PromptMeta:    schema.PromptMeta{Title: cmd.Title, Description: cmd.Description, Input: cmd.Input, Prompt: cmd.Prompt},
		GeneratorMeta: cmd.GeneratorMeta,
	}
	meta.Opt = quoteOpts(meta.Opt)
	return withClient(ctx, "llm.AgentUpdateCmd", func(ctx context.Context, client *httpclient.Client) error {
		agent, err := client.UpdateAgent(ctx, cmd.Name, meta)
		if err != nil {
			return err
		}
		return printAgentYAML(agent.AgentMeta)
	})
}

// Run deletes the agent, and prints its definition, as YAML, as it was, so
// it can be created again
func (cmd *AgentDeleteCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.AgentDeleteCmd", func(ctx context.Context, client *httpclient.Client) error {
		agent, err := client.DeleteAgent(ctx, cmd.Name)
		if err != nil {
			return err
		}
		return printAgentYAML(agent.AgentMeta)
	})
}

// Run prints the agent's definition, as YAML
func (cmd *AgentGetCmd) Run(ctx server.Cmd) error {
	return withClient(ctx, "llm.AgentGetCmd", func(ctx context.Context, client *httpclient.Client) error {
		return client.ReadAgentYAML(ctx, cmd.Name, os.Stdout)
	})
}

// Run creates the agent from its definition, which is read from the file,
// or standard input, and prints it, as YAML
func (cmd *AgentCreateCmd) Run(ctx server.Cmd) error {
	r, err := openDefinition(cmd.File)
	if err != nil {
		return err
	}
	defer r.Close()
	return withClient(ctx, "llm.AgentCreateCmd", func(ctx context.Context, client *httpclient.Client) error {
		agent, err := client.WriteAgentYAML(ctx, r)
		if err != nil {
			return err
		}
		return printAgentYAML(agent.AgentMeta)
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// printAgentYAML prints the agent's definition, as YAML
func printAgentYAML(meta schema.AgentMeta) error {
	data, err := meta.YAML()
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

// openDefinition returns the file with the path, or standard input, when
// the path is empty, or -, unless it's a terminal, as the definition isn't
// typed
func openDefinition(path string) (io.ReadCloser, error) {
	if path != "" && path != "-" {
		return os.Open(path)
	}
	if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return nil, fmt.Errorf("missing file of the agent's definition, or its definition on standard input")
	}
	return io.NopCloser(os.Stdin), nil
}
