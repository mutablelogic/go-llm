package openai

import (
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Format struct {
	// Supported response format types are text, json_object or json_schema
	Type string `json:"type"`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewFormat(format string) *Format {
	format = strings.TrimSpace(strings.ToLower(format))
	switch format {
	case "text", "json_object":
		return &Format{Type: format}
	default:
		// json_schema is not yet supported
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func optQuality(opt *llm.Opts) string {
	return opt.GetString("quality")
}

func optSize(opt *llm.Opts) string {
	return opt.GetString("size")
}

func optStyle(opt *llm.Opts) string {
	return opt.GetString("style")
}
