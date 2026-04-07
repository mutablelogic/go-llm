package tool

import (
	"log/slog"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ToolkitOpt is a functional option for configuring a Toolkit at construction time.
type ToolkitOpt func(*Toolkit) error

///////////////////////////////////////////////////////////////////////////////
// TOOLKIT OPTIONS

// WithLogHandler sets a callback that receives log messages forwarded from
// a connector's MCP session. url identifies the originating connector.
func WithLogHandler(fn func(url string, level slog.Level, msg string, args ...any)) ToolkitOpt {
	return func(tk *Toolkit) error {
		tk.onLog = fn
		return nil
	}
}

// WithStateHandler sets a callback that is invoked when a connector
// successfully connects or reconnects. state contains the server-reported
// name, version, and capabilities for that session.
func WithStateHandler(fn func(url string, state schema.ConnectorState)) ToolkitOpt {
	return func(tk *Toolkit) error {
		tk.onState = fn
		return nil
	}
}

// WithToolsHandler sets a callback that is invoked when a connector's tool
// list changes, or when a connector is removed or disconnects.
// tools is nil when the connector has gone away.
func WithToolsHandler(fn func(url string, tools []llm.Tool)) ToolkitOpt {
	return func(tk *Toolkit) error {
		tk.onTools = fn
		return nil
	}
}

// WithBuiltin adds one or more locally-implemented tools to the toolkit
// at construction time.
func WithBuiltin(tools ...llm.Tool) ToolkitOpt {
	return func(tk *Toolkit) error {
		return tk.AddBuiltin(tools...)
	}
}

///////////////////////////////////////////////////////////////////////////////
// GENERATION OPTIONS

// WithToolkit sets a toolkit for generation options.
// The toolkit is stored under opt.ToolkitKey and can be retrieved
// with opts.Get(opt.ToolkitKey) and type-asserted to *Toolkit.
func WithToolkit(toolkit *Toolkit) opt.Opt {
	return opt.SetAny(opt.ToolkitKey, toolkit)
}

// WithTool adds one or more tools to the generation options.
// Individual tools are appended under opt.ToolKey and merged with
// toolkit tools by each provider.
func WithTool(t ...llm.Tool) opt.Opt {
	return opt.WithTool(t...)
}
