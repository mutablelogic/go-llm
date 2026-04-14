package manager

import (
	"context"
	"fmt"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	mcp "github.com/mutablelogic/go-llm/mcp/client"
	"github.com/mutablelogic/go-llm/pkg/opt"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type delegate struct {
	Name         string
	Version      string
	ClientOpts   []client.ClientOpt
	Connectors   map[string]llm.Connector
	RunAgentFunc runAgentFunc
}

var _ toolkit.ToolkitDelegate = (*delegate)(nil)

type runAgentFunc func(ctx context.Context, prompt llm.Prompt, content string, opts []opt.Opt, resources ...llm.Resource) (llm.Resource, error)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewDelegate(name, version string, connectors map[string]llm.Connector, runagent runAgentFunc, clientopts ...client.ClientOpt) *delegate {
	local := make(map[string]llm.Connector, len(connectors))
	for key, conn := range connectors {
		local[key] = conn
	}
	return &delegate{
		Name:         name,
		Version:      version,
		ClientOpts:   clientopts,
		Connectors:   local,
		RunAgentFunc: runagent,
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - TOOLKIT DELEGATE

// OnEvent is called when a lifecycle or list-change notification is fired.
// ConnectorEventStateChange events are handled internally by the toolkit and
// are never forwarded here. For all other connector-originated events the
// Connector field is set to the originating connector; for builtin add/remove
// operations Connector will be nil.
func (d *delegate) OnEvent(evt toolkit.ConnectorEvent) {
	fmt.Println("Event:", evt.Kind, "Connector:", evt.Connector, "State:", evt.State)
}

// Call executes a prompt via the manager, passing optional input resources.
func (d *delegate) Call(ctx context.Context, prompt llm.Prompt, resources ...llm.Resource) (llm.Resource, error) {
	// Let's prepare the prompt
	content, opts, err := prompt.Prepare(ctx, resources...)
	if err != nil {
		return nil, err
	}

	// Run the agent
	return d.RunAgentFunc(ctx, prompt, content, opts, resources...)
}

// CreateConnector is called to create a new connector for the given reference.
// The onEvent callback must be called by the connector to report lifecycle
// and list-change events back to the toolkit. The toolkit injects the
// Connector field before forwarding to OnEvent, so the caller need not set it.
func (d *delegate) CreateConnector(ref string, onEvent func(evt toolkit.ConnectorEvent)) (llm.Connector, error) {
	fmt.Println("CreateConnector:", ref)
	if conn, exists := d.Connectors[ref]; exists {
		if onEvent != nil {
			onEvent(toolkit.StateChangeEvent(schema.ConnectorState{}))
		}
		return conn, nil
	}

	opts := []mcp.Opt{
		mcp.WithClientOpt(d.ClientOpts...),
	}
	if onEvent != nil {
		opts = append(opts,
			mcp.OptOnStateChange(func(ctx context.Context, state *schema.ConnectorState) {
				onEvent(toolkit.StateChangeEvent(types.Value(state)))
			}),
			mcp.OptOnToolListChanged(func(ctx context.Context) {
				onEvent(toolkit.ToolListChangeEvent())
			}),
			mcp.OptOnPromptListChanged(func(ctx context.Context) {
				onEvent(toolkit.PromptListChangeEvent())
			}),
			mcp.OptOnResourceListChanged(func(ctx context.Context) {
				onEvent(toolkit.ResourceListChangeEvent())
			}),
		)
	}
	return mcp.New(ref, d.Name, d.Version, opts...)
}
