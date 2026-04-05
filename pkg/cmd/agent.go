package cmd

import (
	"fmt"
	"os"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient-new"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type AgentCommands struct {
	ListAgents ListAgentsCommand `cmd:"" name:"agents" help:"List agents." group:"TOOLS & AGENTS"`
	GetAgent   GetAgentCommand   `cmd:"" name:"agent" help:"Get an agent by name." group:"TOOLS & AGENTS"`
}

type ListAgentsCommand struct {
	schema.AgentListRequest `embed:""`
}

type GetAgentCommand struct {
	Name string `arg:"" name:"name" help:"Agent name"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (cmd *ListAgentsCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "ListAgentsCommand",
			attribute.String("request", types.Stringify(cmd.AgentListRequest)),
		)
		defer func() { endSpan(err) }()

		agents, err := client.ListAgents(parent, cmd.AgentListRequest)
		if err != nil {
			return err
		}

		if ctx.IsDebug() {
			fmt.Println(agents)
			return nil
		}

		_, err = tui.TableFor[schema.AgentMeta](tui.SetWidth(ctx.IsTerm())).Write(os.Stdout, agents.Body...)
		return err
	})
}

func (cmd *GetAgentCommand) Run(ctx server.Cmd) (err error) {
	return WithClient(ctx, func(client *httpclient.Client, _ string) error {
		parent, endSpan := otel.StartSpan(ctx.Tracer(), ctx.Context(), "GetAgentCommand",
			attribute.String("name", cmd.Name),
		)
		defer func() { endSpan(err) }()

		agent, err := client.GetAgent(parent, cmd.Name)
		if err != nil {
			return err
		}

		fmt.Println(agent)
		return nil
	})
}
