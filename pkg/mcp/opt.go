package mcp

import "github.com/mutablelogic/go-llm/pkg/tool"

/////////////////////////////////////////////////////////////////////////////////
// TYPES

type Opt func(*Server) error

/////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (server *Server) apply(opts ...Opt) error {
	for _, opt := range opts {
		if err := opt(server); err != nil {
			return err
		}
	}
	return nil
}

/////////////////////////////////////////////////////////////////////////////////
// OPTIONS

func WithToolKit(v *tool.Toolkit) Opt {
	return func(server *Server) error {
		server.toolkit = v
		return nil
	}
}
