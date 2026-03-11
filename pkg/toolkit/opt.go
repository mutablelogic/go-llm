package toolkit

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Option configures a Toolkit at construction time.
type Option func(*toolkit) error

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithTool registers one or more builtin tools with the toolkit at construction time.
func WithTool(items ...llm.Tool) Option {
	return func(*toolkit) error {
		return llm.ErrNotImplemented
	}
}

// WithPrompt registers one or more builtin prompts with the toolkit at construction time.
func WithPrompt(items ...llm.Prompt) Option {
	return func(*toolkit) error {
		return llm.ErrNotImplemented
	}
}

// WithResource registers one or more builtin resources with the toolkit at construction time.
func WithResource(items ...llm.Resource) Option {
	return func(*toolkit) error {
		return llm.ErrNotImplemented
	}
}

// WithHandler sets the ToolkitHandler that receives connector lifecycle callbacks,
// executes prompts, serves the "user" namespace, and creates connectors.
func WithHandler(h ToolkitHandler) Option {
	return func(*toolkit) error {
		return llm.ErrNotImplemented
	}
}

// WithTracer sets an OpenTelemetry tracer. The toolkit starts a span named after
// the tool before each Run call and embeds it into the ctx.
func WithTracer(t trace.Tracer) Option {
	return func(*toolkit) error {
		return llm.ErrNotImplemented
	}
}
