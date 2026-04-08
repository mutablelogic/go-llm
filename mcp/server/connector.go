package server

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddConnector registers all tools, prompts, and resources currently exposed
// by an llm.Connector on this MCP server.
//
// AddConnector does not call conn.Run. Callers are responsible for managing
// the connector lifecycle when working with connectors that require an active
// background session.
func (s *Server) AddConnector(ctx context.Context, conn llm.Connector) error {
	if conn == nil {
		return nil
	}

	// Add tools
	if tools, err := conn.ListTools(ctx); err != nil {
		return err
	} else if err := s.AddTools(tools...); err != nil {
		return err
	}

	// Add prompts
	if prompts, err := conn.ListPrompts(ctx); err != nil {
		return err
	} else {
		s.AddPrompts(prompts...)
	}

	// Add resources
	if resources, err := conn.ListResources(ctx); err != nil {
		return err
	} else if err := s.AddResources(resources...); err != nil {
		return err
	}

	// Return success
	return nil
}
