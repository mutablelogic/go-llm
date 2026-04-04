package manager

import (
	"context"
	"maps"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (m *Manager) CreateUsage(ctx context.Context, req schema.UsageInsert) (_ *schema.Usage, err error) {
	// Otel span
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "Ask",
		attribute.String("req", types.Stringify(req)),
	)
	defer func() { endSpan(err) }()

	// Insert the usage record
	var result schema.Usage
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		return conn.Insert(ctx, &result, req)
	}); err != nil {
		return nil, err
	}

	// Return success
	return types.Ptr(result), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// mergeUsageMeta combines usage metadata with configured provider metadata,
// provider metadata returned on the generated message, and the current trace_id.
// Configured provider metadata is applied first, message metadata overrides any
// overlapping keys, and trace_id is added last when a valid span is present.
func mergeUsageMeta(ctx context.Context, usage *schema.UsageMeta, providerMeta schema.ProviderMetaMap, message *schema.Message) *schema.UsageMeta {
	hasProviderMeta := message != nil && len(message.Meta) > 0
	hasConfiguredProviderMeta := len(providerMeta) > 0
	hasTraceID := trace.SpanContextFromContext(ctx).IsValid()
	if usage == nil && !hasProviderMeta && !hasConfiguredProviderMeta {
		return nil
	}
	if usage == nil {
		usage = new(schema.UsageMeta)
	}
	if usage.Meta == nil && (hasConfiguredProviderMeta || hasProviderMeta || hasTraceID) {
		usage.Meta = make(schema.ProviderMetaMap)
	}
	if hasConfiguredProviderMeta {
		maps.Copy(usage.Meta, providerMeta)
	}
	if hasProviderMeta {
		maps.Copy(usage.Meta, message.Meta)
	}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		usage.Meta["trace_id"] = span.TraceID().String()
	}
	return usage
}
