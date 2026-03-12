package testdata

import "embed"

//go:embed *.md
//go:exclude README.md
var FS embed.FS
