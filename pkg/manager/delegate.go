package manager

import (
	"context"
	"fmt"

	// Packages

	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	mcp "github.com/mutablelogic/go-llm/pkg/mcp/client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	toolkit "github.com/mutablelogic/go-llm/pkg/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type delegate struct {
	Name       string
	Version    string
	ClientOpts []client.ClientOpt
}

var _ toolkit.ToolkitDelegate = (*delegate)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewDelegate(name, version string, clientopts ...client.ClientOpt) *delegate {
	return &delegate{
		Name:       name,
		Version:    version,
		ClientOpts: clientopts,
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
	// Let's prepare the prompt and options, then return an error since we haven't implemented this yet
	// TODO
	content, opts, err := prompt.Prepare(ctx, nil)
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("Call is not implemented with content %v and options %v", content, opts)
}

// CreateConnector is called to create a new connector for the given URL.
// The onEvent callback must be called by the connector to report lifecycle
// and list-change events back to the toolkit. The toolkit injects the
// Connector field before forwarding to OnEvent, so the caller need not set it.
func (d *delegate) CreateConnector(url string, onEvent func(evt toolkit.ConnectorEvent)) (llm.Connector, error) {
	fmt.Println("CreateConnector:", url)

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
	return mcp.New(url, d.Name, d.Version, opts...)
}
