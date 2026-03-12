package testdata

import "embed"

//go:embed *
//go:exclude *.go
var FS embed.FS
