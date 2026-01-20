package server

import "github.com/mutablelogic/go-llm"

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

func WithToolKit(v llm.ToolKit) Opt {
	return func(server *Server) error {
		server.toolkit = v
		return nil
	}
}
