//go:build !client && !frontend

package cmd

import (
	// Packages
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

// builtIn is whether the web UI is built in, which it isn't, without the
// "frontend" tag
const builtIn = false

// registerFS does nothing, as the web UI isn't built in
func registerFS(*httprouter.Router) error {
	return nil
}
