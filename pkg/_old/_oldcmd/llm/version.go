package main

import (
	"fmt"
	"os"
	"runtime"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/version"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type VersionCmd struct{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run the version command
func (cmd *VersionCmd) Run() error {
	w := os.Stdout
	if version.GitSource != "" {
		fmt.Fprintf(w, "Source: https://%v\n", version.GitSource)
	}
	if version.GitTag != "" || version.GitBranch != "" {
		fmt.Fprintf(w, "Version: %v (branch: %q hash:%q)\n", version.GitTag, version.GitBranch, version.GitHash)
	}
	if version.GoBuildTime != "" {
		fmt.Fprintf(w, "Build: %v\n", version.GoBuildTime)
	}
	fmt.Fprintf(w, "Go: %v (%v/%v)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return nil
}
