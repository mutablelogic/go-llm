package client

import (
	"context"
	"time"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Probe makes a single MCP connection, reads the initialisation result
// (server name, version, instructions and capabilities), closes the session
// and returns the collected data as a ConnectorState ready to persist.
// It never leaves a persistent session open.
//
// The provided ctx governs the connection timeout; cancelling it before the
// server responds causes Probe to return ctx.Err().
func (c *Client) Probe(ctx context.Context) (*schema.ConnectorState, error) {
	// Establish a session (includes auth retry if c.authFn is set).
	session, err := c.connectWithAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Always close the transport when we return so the probe is a clean
	// one-shot with no persistent side-effects.
	defer session.Close()

	// Create a connector state object
	now := time.Now()
	state := &schema.ConnectorState{
		ConnectedAt: types.Ptr(now),
	}

	// Call initialize
	if res := session.InitializeResult(); res != nil {
		if res.ServerInfo != nil {
			if res.ServerInfo.Name != "" {
				state.Name = types.Ptr(res.ServerInfo.Name)
			}
			if res.ServerInfo.Title != "" {
				state.Title = types.Ptr(res.ServerInfo.Title)
			}
			if res.ServerInfo.Version != "" {
				state.Version = types.Ptr(res.ServerInfo.Version)
			}
		}
		if res.Instructions != "" {
			state.Description = types.Ptr(res.Instructions)
		}
		if caps := res.Capabilities; caps != nil {
			if caps.Tools != nil {
				state.Capabilities = append(state.Capabilities, schema.CapabilityTools)
			}
			if caps.Resources != nil {
				state.Capabilities = append(state.Capabilities, schema.CapabilityResources)
			}
			if caps.Prompts != nil {
				state.Capabilities = append(state.Capabilities, schema.CapabilityPrompts)
			}
			if caps.Logging != nil {
				state.Capabilities = append(state.Capabilities, schema.CapabilityLogging)
			}
			if caps.Completions != nil {
				state.Capabilities = append(state.Capabilities, schema.CapabilityCompletions)
			}
		}
	}

	return state, nil
}
