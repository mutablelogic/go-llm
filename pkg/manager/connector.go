package manager

import (
	"context"
	"errors"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

// prober is satisfied by connectors that support a one-shot probe for
// server metadata (name, version, capabilities). *mcpclient.Client implements this.
type prober interface {
	Probe(ctx context.Context) (*schema.ConnectorState, error)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateConnector registers a new MCP connector and persists its metadata.
// When a connector factory is configured and the connector supports probing,
// the server state (name, version, capabilities) is fetched synchronously and
// persisted before returning. Subsequent reconnects update the state
// asynchronously via the background session.
func (m *Manager) CreateConnector(ctx context.Context, rawURL string, meta schema.ConnectorMeta) (result *schema.Connector, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateConnector",
		attribute.String("url", rawURL),
		attribute.String("meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	// Check incoming parameters
	url, err := schema.CanonicalURL(rawURL)
	if err != nil {
		return nil, llm.ErrBadParameter.With(err)
	} else if ns := types.Value(meta.Namespace); ns != "" && !types.IsIdentifier(ns) {
		return nil, llm.ErrBadParameter.Withf("connector namespace %q is not a valid identifier", ns)
	}

	// If a connector factory is configured, use it to probe the server before
	// registering so that name, version and capabilities are persisted immediately.
	var state *schema.ConnectorState
	var conn llm.Connector
	if m.connectorFactory != nil {
		conn, err = m.connectorFactory(ctx, url, m.credOptsFor(ctx, url)...)
		if err != nil {
			return nil, err
		}

		if p, ok := conn.(prober); ok {
			state, err = p.Probe(ctx)
			if err != nil {
				return nil, err
			}

			// Derive namespace from server name if not provided.
			if types.Value(meta.Namespace) == "" && types.Value(state.Name) != "" {
				meta.Namespace = types.Ptr(schema.CanonicalNamespace(types.Value(state.Name)))
			}
		}
	}

	// Create the connector
	result, err = m.connectorStore.CreateConnector(ctx, url, meta)
	if err != nil {
		return nil, err
	}

	// Persist the probed state if we have one; roll back the registration on failure.
	if state != nil {
		result, err = m.connectorStore.UpdateConnectorState(ctx, url, *state)
		if err != nil {
			return nil, errors.Join(err, m.connectorStore.DeleteConnector(ctx, url))
		}
	}

	// Wire into the toolkit so the background session starts.
	// Roll back the store entry if the toolkit rejects it (e.g. duplicate URL).
	if conn != nil && types.Value(result.Enabled) {
		if err = m.toolkit.AddConnector(url, conn); err != nil {
			return nil, errors.Join(err, m.connectorStore.DeleteConnector(ctx, url))
		}
	}

	// Return success
	return result, nil
}

// GetConnector returns the connector for the given URL.
func (m *Manager) GetConnector(ctx context.Context, rawURL string) (result *schema.Connector, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "GetConnector",
		attribute.String("url", rawURL),
	)
	defer func() { endSpan(err) }()

	url, err := schema.CanonicalURL(rawURL)
	if err != nil {
		return nil, llm.ErrBadParameter.With(err)
	}

	return m.connectorStore.GetConnector(ctx, url)
}

// UpdateConnector updates the user-editable metadata for an existing connector.
func (m *Manager) UpdateConnector(ctx context.Context, rawURL string, meta schema.ConnectorMeta) (result *schema.Connector, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "UpdateConnector",
		attribute.String("url", rawURL),
		attribute.String("meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	url, err := schema.CanonicalURL(rawURL)
	if err != nil {
		return nil, llm.ErrBadParameter.With(err)
	}
	if ns := types.Value(meta.Namespace); ns != "" && !types.IsIdentifier(ns) {
		return nil, llm.ErrBadParameter.Withf("connector namespace %q is not a valid identifier", ns)
	}

	result, err = m.connectorStore.UpdateConnector(ctx, url, meta)
	if err != nil {
		return nil, err
	}

	// If the enabled flag was explicitly changed, wire or unwire the toolkit.
	if meta.Enabled != nil {
		if types.Value(result.Enabled) && m.connectorFactory != nil {
			// Always remove first so a reconnect gets a fresh session.
			m.toolkit.RemoveConnector(url)
			conn, connErr := m.connectorFactory(ctx, url, m.credOptsFor(ctx, url)...)
			if connErr != nil {
				return nil, connErr
			}
			if err = m.toolkit.AddConnector(url, conn); err != nil {
				return nil, err
			}
		} else {
			m.toolkit.RemoveConnector(url)
		}
	}

	return result, nil
}

// DeleteConnector removes the connector for the given URL.
func (m *Manager) DeleteConnector(ctx context.Context, rawURL string) (err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "DeleteConnector",
		attribute.String("url", rawURL),
	)
	defer func() { endSpan(err) }()

	url, err := schema.CanonicalURL(rawURL)
	if err != nil {
		return llm.ErrBadParameter.With(err)
	}

	// Disconnect from the toolkit before removing from the store.
	m.toolkit.RemoveConnector(url)

	return m.connectorStore.DeleteConnector(ctx, url)
}

// ListConnectors returns connectors matching the request filters.
func (m *Manager) ListConnectors(ctx context.Context, req schema.ListConnectorsRequest) (result *schema.ListConnectorsResponse, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListConnectors",
		attribute.String("req", req.String()),
	)
	defer func() { endSpan(err) }()

	return m.connectorStore.ListConnectors(ctx, req)
}
