package manager

import (
	"context"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateConnector registers a new MCP connector and persists its metadata.
// The connector state (server name, version, capabilities) will be populated
// asynchronously once the background session establishes.
func (m *Manager) CreateConnector(ctx context.Context, rawURL string, meta schema.ConnectorMeta) (result *schema.Connector, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateConnector",
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

	return m.connectorStore.CreateConnector(ctx, url, meta)
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

	return m.connectorStore.UpdateConnector(ctx, url, meta)
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
