package version

import (
	"encoding/json"
	"runtime"
	"runtime/debug"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	GitTag    string
	GitBranch string
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
	// Fall back to vcs.revision from build info
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				return s.Value[:12]
			}
		}
	}
	return "dev"
}

func JSON(execName string) []byte {
	metadata := map[string]string{
		"name":     execName,
		"version":  Version(),
		"compiler": runtime.Version(),
	}

	// Add ldflags values if set
	if GitTag != "" {
		metadata["tag"] = GitTag
	}
	if GitBranch != "" {
		metadata["branch"] = GitBranch
	}

	// Add build info from runtime/debug
	var goos, goarch string
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Path != "" {
			metadata["source"] = info.Main.Path
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if s.Value != "" {
					metadata["hash"] = s.Value
				}
			case "vcs.time":
				if s.Value != "" {
					metadata["build_time"] = s.Value
				}
			case "vcs.modified":
				if s.Value == "true" {
					metadata["modified"] = s.Value
				}
			case "GOOS":
				goos = s.Value
			case "GOARCH":
				goarch = s.Value
			}
		}
	}
	if goos != "" && goarch != "" {
		metadata["platform"] = goos + "/" + goarch
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		panic(err)
	}
	return data
}
