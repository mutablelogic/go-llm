package connector

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	heartbeat "github.com/mutablelogic/go-llm/heartbeat/manager"
	pg "github.com/mutablelogic/go-pg"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Connector struct {
	*heartbeat.Manager
}

var _ llm.Connector = (*Connector)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func New(ctx context.Context, pool pg.PoolConn, opts ...heartbeat.Opt) (*Connector, error) {
	manager, err := heartbeat.New(ctx, pool, opts...)
	if err != nil {
		return nil, err
	}
	return &Connector{Manager: manager}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTools satisfies the llm.Connector interface.
func (c *Connector) ListTools(_ context.Context) ([]llm.Tool, error) {
	return []llm.Tool{
		c.NewCreateSchedule(), c.NewListSchedule(),
	}, nil
}

// ListPrompts satisfies the llm.Connector interface.
func (c *Connector) ListPrompts(context.Context) ([]llm.Prompt, error) {
	return nil, nil
}

// ListResources satisfies the llm.Connector interface.
func (c *Connector) ListResources(context.Context) ([]llm.Resource, error) {
	return nil, nil
}

// Run starts the maturity-check loop and blocks until ctx is cancelled.
func (c *Connector) Run(ctx context.Context) error {
	return c.Manager.Run(ctx)
}
