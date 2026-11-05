//go:build !client && frontend

package cmd

import (
	"io/fs"
	"mime"

	// Packages
	frontend "github.com/mutablelogic/go-llm/build/frontend"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
)

// builtIn is whether the web UI is built in, which it is, with the
// "frontend" tag
const builtIn = true

// registerFS registers the web UI, from its root. Its fonts are served with
// their type, which Go doesn't know, and which a system's own types mayn't
// have.
func registerFS(router *httprouter.Router) error {
	if err := mime.AddExtensionType(".woff2", "font/woff2"); err != nil {
		return err
	}
	sub, err := fs.Sub(frontend.EmbedFS, "dist")
	if err != nil {
		return err
	}
	return router.RegisterFS("/", sub, false, nil)
}
