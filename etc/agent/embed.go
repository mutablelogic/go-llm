package testdata

import "embed"

//go:embed extract_entities.md summarize.md translate.md
var FS embed.FS
