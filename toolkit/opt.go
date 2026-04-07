package toolkit

import (
	// Packages
	"log/slog"

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
	return func(tk *toolkit) error {
		return tk.AddTool(items...)
	}
}

// WithPrompt registers one or more builtin prompts with the toolkit at construction time.
func WithPrompt(items ...llm.Prompt) Option {
	return func(tk *toolkit) error {
		return tk.AddPrompt(items...)
	}
}

// WithResource registers one or more builtin resources with the toolkit at construction time.
func WithResource(items ...llm.Resource) Option {
	return func(tk *toolkit) error {
		return tk.AddResource(items...)
	}
}

// WithDelegate sets the ToolkitDelegate that receives connector lifecycle callbacks,
// executes prompts, serves the "user" namespace, and creates connectors.
func WithDelegate(h ToolkitDelegate) Option {
	return func(tk *toolkit) error {
		tk.delegate = h
		return nil
	}
}

// WithTracer sets an OpenTelemetry tracer. The toolkit starts a span named after
// the tool before each Run call and embeds it into the ctx.
func WithTracer(t trace.Tracer) Option {
	return func(tk *toolkit) error {
		tk.tracer = t
		return nil
	}
}

// WithLogger sets a slog.Logger for the toolkit to use for logging.
func WithLogger(l *slog.Logger) Option {
	return func(tk *toolkit) error {
		if l == nil {
			tk.logger = slog.Default()
		} else {
			tk.logger = l
		}
		return nil
	}
}
