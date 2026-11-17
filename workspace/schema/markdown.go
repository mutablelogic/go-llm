package schema

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Markdown is a part of a resource, read as Markdown: its body, from the
// byte of its Markdown at the offset, the offset of the next part, which is
// left out at its end, and, with the first part, its metadata, by namespace
// and key, such as dc:title
type Markdown struct {
	URI    string            `json:"uri" help:"URI of the resource"`
	Type   string            `json:"type,omitempty" help:"MIME type of the resource"`
	Body   string            `json:"body" help:"The part's Markdown"`
	Offset uint64            `json:"offset" help:"The byte of the Markdown the part starts at"`
	Next   *uint64           `json:"next,omitempty" help:"The offset of the next part, or none, at the end"`
	Meta   map[string]string `json:"meta,omitempty" help:"The resource's metadata, with the first part, by namespace and key, such as dc:title"`
}
