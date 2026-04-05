package manager

import (
	"context"
	"errors"
	"strings"

	// Packages
	authclient "github.com/djthorpe/go-auth/pkg/httpclient/auth"
	"github.com/djthorpe/go-auth/pkg/oidc"
	auth "github.com/djthorpe/go-auth/schema/auth"
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	"github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// We define the MCP client prober here, so we can get the details of an MCP Server
// before we insert it into the database. If there is a probe authentication failure,
// we can return the details of the auth failure to the client so they can fix it before retrying.
type ConnectorProbe interface {
	Probe(ctx context.Context, auth func(err error, config *authclient.Config) error) (*schema.ConnectorState, error)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateConnector validates and persists a connector insert request.
func (m *Manager) CreateConnector(ctx context.Context, req schema.ConnectorInsert, user *auth.User) (_ *schema.Connector, _ *oidc.BaseConfiguration, _ []string, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateConnector",
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// Canonicalize the URL
	if url, err := schema.CanonicalURL(req.URL); err != nil {
		return nil, nil, nil, schema.ErrBadParameter.Withf("invalid connector URL: %v", err)
	} else {
		req.URL = url
	}

	// Create an MCP client so we can retrieve all the information we need to validate the MCP server
	// Here we use the *auth.User credentials for authentication, when they have been implemented.
	var state schema.ConnectorState
	var scopes []string
	var codeflow *oidc.BaseConfiguration
	if connector, err := m.delegate.CreateConnector(req.URL, nil); err != nil {
		return nil, nil, nil, schema.ErrBadParameter.Withf("failed to validate connector URL: %v", err)
	} else if state_, err := connector.(ConnectorProbe).Probe(ctx, func(authErr error, config *authclient.Config) error {
		var err error
		codeflow, scopes, err = connectorUnauthorizedDetail(config)
		if err != nil {
			return err
		}
		return connectorUnauthorizedError(authErr)
	}); errors.Is(err, httpresponse.ErrNotAuthorized) {
		// Return the code flow configuration and the supported scopes (if any) in the error details
		return nil, codeflow, scopes, err
	} else if err != nil {
		return nil, nil, nil, err
	} else {
		state = types.Value(state_)
	}

	// We set the namespace for the connector
	if types.Value(req.Namespace) == "" {
		if state := strings.TrimSpace(types.Value(state.Name)); state != "" {
			req.Namespace = types.Ptr(schema.CanonicalNamespace(state))
		}
	}
	if types.Value(req.Namespace) == "" {
		return nil, nil, nil, schema.ErrBadParameter.With("connector namespace is required")
	}

	// Insert the connector record, then sync the groups if provided and return the result
	var result schema.Connector
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		var inserted schema.Connector
		if err := conn.Insert(ctx, &inserted, req); err != nil {
			return err
		}
		if req.Groups != nil {
			if err := m.syncConnectorGroups(ctx, conn, inserted.URL, req.Groups); err != nil {
				return err
			}
		}
		if state.HasTableUpdates() {
			var updated schema.Connector
			if err := conn.Update(ctx, &updated, schema.ConnectorStateSelector(inserted.URL), state); err != nil {
				return err
			}
		}
		return conn.Get(ctx, &result, schema.ConnectorURLSelector(inserted.URL))
	}); err != nil {
		return nil, nil, nil, pg.NormalizeError(err)
	}

	// Return success
	return types.Ptr(result), nil, nil, nil
}

func connectorUnauthorizedDetail(config *authclient.Config) (*oidc.BaseConfiguration, []string, error) {
	codeConfig, err := config.AuthorizationCodeConfig()
	if err != nil {
		return nil, nil, err
	}
	return types.Ptr(codeConfig), config.ScopesSupported, nil
}

func connectorUnauthorizedError(err error) error {
	if err == nil {
		return httpresponse.ErrNotAuthorized
	}
	if authErr := authclient.AsAuthError(err); authErr != nil {
		reason := strings.TrimSpace(authErr.Error())
		if reason == "" || strings.EqualFold(reason, "unauthorized") {
			return httpresponse.ErrNotAuthorized
		}
		return httpresponse.ErrNotAuthorized.With(reason)
	}
	return httpresponse.ErrNotAuthorized.With(err)
}

// DeleteConnector removes the connector for the given URL and returns the deleted connector.
func (m *Manager) DeleteConnector(ctx context.Context, url string) (_ *schema.Connector, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "DeleteConnector",
		attribute.String("url", url),
	)
	defer func() { endSpan(err) }()

	var deleted schema.Connector
	if err := m.PoolConn.Delete(ctx, &deleted, schema.ConnectorURLSelector(url)); err != nil {
		return nil, normalizeConnectorError(url, err)
	}

	return types.Ptr(deleted), nil
}

// GetConnector returns the connector for the given URL and, when user is set,
// scopes access to public connectors or those accessible to the user's groups.
func (m *Manager) GetConnector(ctx context.Context, url string, user *auth.User) (_ *schema.Connector, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "GetConnector",
		attribute.String("url", url),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	var result schema.Connector
	var conn pg.Conn = m.PoolConn
	if user != nil {
		conn = conn.With("user", user.UUID())
	}
	if err := conn.Get(ctx, &result, schema.ConnectorURLSelector(url)); err != nil {
		return nil, normalizeConnectorError(url, err)
	}

	return types.Ptr(result), nil
}

// UpdateConnector updates the user-editable metadata for the connector and
// returns the updated connector.
func (m *Manager) UpdateConnector(ctx context.Context, url string, meta schema.ConnectorMeta) (_ *schema.Connector, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "UpdateConnector",
		attribute.String("url", url),
		attribute.String("meta", meta.String()),
	)
	defer func() { endSpan(err) }()

	if !meta.HasTableUpdates() && meta.Groups == nil {
		return nil, schema.ErrBadParameter.With("no fields to update")
	}

	var result schema.Connector
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		selector := schema.ConnectorURLSelector(url)
		if meta.HasTableUpdates() {
			var updated schema.Connector
			if err := conn.Update(ctx, &updated, selector, meta); err != nil {
				return err
			}
		} else {
			if err := conn.Get(ctx, &result, selector); err != nil {
				return err
			}
		}
		if meta.Groups != nil {
			if err := m.syncConnectorGroups(ctx, conn, url, meta.Groups); err != nil {
				return err
			}
		}
		return conn.Get(ctx, &result, selector)
	}); err != nil {
		return nil, normalizeConnectorError(url, err)
	}

	return types.Ptr(result), nil
}

// ListConnectors lists connectors matching the request and, when user is set,
// filters results to public connectors or those accessible to the user's groups.
func (m *Manager) ListConnectors(ctx context.Context, req schema.ConnectorListRequest, user *auth.User) (_ *schema.ConnectorList, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "ListConnectors",
		attribute.String("req", req.String()),
		attribute.String("user", types.Stringify(user)),
	)
	defer func() { endSpan(err) }()

	// List connectors matching the request and user context
	result := schema.ConnectorList{ConnectorListRequest: req}
	var conn pg.Conn = m.PoolConn
	if user != nil {
		conn = conn.With("user", user.UUID())
	}
	if err := conn.List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit = req.OffsetLimit
	result.OffsetLimit.Clamp(uint64(result.Count))

	// Return success
	return types.Ptr(result), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *Manager) syncConnectorGroups(ctx context.Context, conn pg.Conn, connector string, groups []string) error {
	var deleted schema.ConnectorGroupList
	if err := conn.Delete(ctx, &deleted, schema.ConnectorGroupSelector{Connector: connector}); err != nil && !errors.Is(err, pg.ErrNotFound) {
		return err
	}

	for _, group := range groups {
		var inserted schema.ConnectorGroupList
		if err := conn.Insert(ctx, &inserted, schema.ConnectorGroupRef{Connector: connector, Group: group}); err != nil && !errors.Is(err, pg.ErrNotFound) {
			return err
		}
	}

	return nil
}

func normalizeConnectorError(url string, err error) error {
	err = pg.NormalizeError(err)
	if errors.Is(err, pg.ErrNotFound) || errors.Is(err, schema.ErrNotFound) {
		return schema.ErrNotFound.Withf("connector %q", url)
	}
	return err
}
