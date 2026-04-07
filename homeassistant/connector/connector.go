package homeassistant

import (
	"context"

	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/homeassistant/httpclient"
	"github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type connector struct {
	tools []llm.Tool
}

var _ llm.Connector = (*connector)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new Connector for the given endpoint and API key for a
// Home Assistant server.
func New(endPoint, apiKey string, opts ...client.ClientOpt) (llm.Connector, error) {
	self := new(connector)

	client, err := httpclient.New(endPoint, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	// Add the tools
	self.tools = append(self.tools, types.Ptr(getStates{client: client}))
	self.tools = append(self.tools, types.Ptr(getState{client: client}))
	self.tools = append(self.tools, types.Ptr(callService{client: client}))
	self.tools = append(self.tools, types.Ptr(getServices{client: client}))
	self.tools = append(self.tools, types.Ptr(setState{client: client}))
	self.tools = append(self.tools, types.Ptr(fireEvent{client: client}))
	self.tools = append(self.tools, types.Ptr(renderTemplate{client: client}))

	// Return the connector
	return self, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run establishes and drives the connection until ctx is cancelled
func (c *connector) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// ListTools returns all tools advertised by the connected remote server.
func (c *connector) ListTools(ctx context.Context) ([]llm.Tool, error) {
	return c.tools, nil
}

// ListPrompts returns all prompts advertised by the connected remote server.
func (c *connector) ListPrompts(ctx context.Context) ([]llm.Prompt, error) {
	return nil, nil
}

// ListResources returns all resources advertised by the connected remote server.
func (c *connector) ListResources(ctx context.Context) ([]llm.Resource, error) {
	return nil, nil
}
