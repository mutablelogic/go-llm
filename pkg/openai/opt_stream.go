package openai

///////////////////////////////////////////////////////////////////////////////
// TYPES

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewStreamOptions(include_usage bool) *StreamOptions {
	return &StreamOptions{IncludeUsage: include_usage}
}
