package llm

// SetMaxTextBytes sets the most of a document of text which go-metadata
// converts to Markdown, and returns what it was
func SetMaxTextBytes(n int64) int64 {
	was := maxTextBytes
	maxTextBytes = n
	return was
}
