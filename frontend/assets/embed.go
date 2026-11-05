//go:build frontend

// Package frontend is the web UI, which the server embeds, with the
// "frontend" tag. It's copied to build/frontend by build.js, with the UI it
// builds in dist.
package frontend

import "embed"

// EmbedFS is the UI, in dist
//
//go:embed dist
var EmbedFS embed.FS
