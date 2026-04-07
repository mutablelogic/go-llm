package store

import (
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - CONNECTOR UTILITIES

// validateConnectorNamespace checks that namespace is either empty or a valid
// identifier as defined by types.IsIdentifier.
func validateConnectorNamespace(namespace string) error {
	if namespace == "" {
		return nil
	}
	if !types.IsIdentifier(namespace) {
		return schema.ErrBadParameter.Withf("connector namespace: must be a valid identifier or empty, got %q", namespace)
	}
	return nil
}
