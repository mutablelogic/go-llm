package schema

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

// BuiltinNamespace is the namespace used for locally-implemented (builtin) tools.
const BuiltinNamespace = "builtin"

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ListToolsRequest specifies optional filters for the Toolkit.ListTools method.
// Zero value returns all tools from all namespaces.
type ListToolsRequest struct {
	// Namespace restricts results to a single namespace.
	// Use BuiltinNamespace for locally-implemented tools, a connector URL for
	// remote tools, or leave empty to include all namespaces.
	Namespace string `json:"namespace,omitempty"`

	// Name restricts results to tools whose names appear in this list.
	// An empty slice means no name filter — all names are included.
	Name []string `json:"name,omitempty"`
}
