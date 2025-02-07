package openai

import "strings"

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
