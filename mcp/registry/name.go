package registry

import (
	"context"
	"errors"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	authclient "github.com/mutablelogic/go-llm/auth/httpclient"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// maxNames is how many numbered names a connector is tried with, such as
// github-2, when it's named after its server, and the name is another's
const maxNames = 100

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// createNamed creates the connector, which has no name, named after its
// server, and with a number added, such as github-2, when the name is
// another connector's
func (r *Registry) createNamed(ctx context.Context, connector schema.Connector) (*schema.Connector, error) {
	if err := connector.ConnectorMeta.Validate(); err != nil {
		return nil, err
	}
	base := schema.ConnectorName(r.serverName(ctx, connector.ConnectorMeta))
	for n := 1; n <= maxNames; n++ {
		connector.Name = schema.NumberedName(base, n)
		created, err := r.create(ctx, connector)
		if !errors.Is(err, llm.ErrConflict) {
			return created, err
		}
	}
	return nil, llm.ErrConflict.Withf("connectors named %q, and numbered, exist already", base)
}

// serverName returns the name of the connector's server, as it says, once
// it's connected, or as the metadata of how it's authorized says, when it
// needs to be, as it can't be connected before then, or otherwise the
// host of its URL, such as when it can't be connected
func (r *Registry) serverName(ctx context.Context, meta schema.ConnectorMeta) string {
	c, err := r.newClient(schema.Connector{ConnectorMeta: meta})
	if err != nil {
		return schema.HostName(meta.URL)
	}
	defer func() { _ = c.Disconnect() }()
	connectCtx, cancel := context.WithTimeout(ctx, r.connectTimeout)
	defer cancel()
	err = c.Connect(connectCtx)
	switch {
	case err == nil:
		if server, _ := c.Status(); server != nil && server.Name != "" {
			return server.Name
		}
	case authclient.AsAuthError(err) != nil:
		if metadata, err := c.DiscoverWithError(connectCtx, err); err == nil && metadata.Resource.ResourceName != "" {
			return metadata.Resource.ResourceName
		}
	default:
		r.log().Debug("connector's server can't be connected, so it's named after its host", "url", meta.URL, "error", err.Error())
	}
	return schema.HostName(meta.URL)
}
