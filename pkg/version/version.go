package version

import (
	"encoding/json"
	"runtime"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	GitSource   string
	GitTag      string
	GitBranch   string
	GitHash     string
	GoBuildTime string
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func Version() string {
	if GitTag != "" {
		return GitTag
	}
	if GitBranch != "" {
		return GitBranch
	}
	if GitHash != "" {
		return GitHash
	}
	return "dev"
}

func JSON(execName string) []byte {
	metadata := map[string]string{
		"name":       execName,
		"compiler":   runtime.Version(),
		"source":     GitSource,
		"tag":        GitTag,
		"branch":     GitBranch,
		"hash":       GitHash,
		"build_time": GoBuildTime,
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		panic(err)
	}
	return data
}
