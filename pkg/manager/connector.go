package manager

import (
	"context"
	"errors"

	// Packages
	client "github.com/mutablelogic/go-client"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	mcpclient "github.com/mutablelogic/go-llm/pkg/mcp/client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateConnector probes the MCP server first, then registers the connector
// and persists its state. If the state update fails the registration is rolled
// back and the error is returned.
func (m *Manager) CreateConnector(ctx context.Context, rawURL string, meta schema.ConnectorMeta) (result *schema.Connector, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateConnector",
		attribute.String("url", rawURL),
		attribute.String("meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	url, err := schema.CanonicalURL(rawURL)
	if err != nil {
		return nil, err
	}
	if ns := meta.Namespace; ns != "" && !types.IsIdentifier(ns) {
		return nil, llm.ErrBadParameter.Withf("connector namespace %q is not a valid identifier", ns)
	}

	// Probe the MCP server before registering it, injecting a bearer token if
	// the credential store holds one for this server.
	var probeOpts []client.ClientOpt
	if m.tracer != nil {
		probeOpts = append(probeOpts, client.OptTracer(m.tracer))
	}
	if m.credentialStore != nil {
		if cred, credErr := m.credentialStore.GetCredential(ctx, url); credErr == nil && cred.Token != nil && cred.Token.AccessToken != "" {
			probeOpts = append(probeOpts, client.OptReqToken(client.Token{Scheme: "Bearer", Value: cred.Token.AccessToken}))
		}
	}

	mcpClient, err := mcpclient.New(url, m.serverName, m.serverVersion, nil, probeOpts...)
	if err != nil {
		return nil, err
	}
	state, err := mcpClient.Probe(ctx)
	if err != nil {
		return nil, err
	}

	// Probe succeeded — register the connector, then persist its state.
	// If UpdateConnectorState fails, roll back by deleting the connector.
	// If no namespace was provided, derive one from the server's name.
	if meta.Namespace == "" && state.Name != nil && *state.Name != "" {
		meta.Namespace = schema.CanonicalNamespace(*state.Name)
	}
	result, err = m.connectorStore.CreateConnector(ctx, url, meta)
	if err != nil {
		return nil, err
	}

	// Update the connector state with the probe results. If this fails, delete the connector to roll back and return the error.
	result, err = m.connectorStore.UpdateConnectorState(ctx, url, *state)
	if err != nil {
		return nil, errors.Join(err, m.connectorStore.DeleteConnector(ctx, url))
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
		return nil, err
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
		return nil, err
	}
	if ns := meta.Namespace; ns != "" && !types.IsIdentifier(ns) {
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
		return err
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
