package schema

// MetaValue is a single key/value pair for the MCP _meta field.
type MetaValue struct {
	Key   string
	Value any
}

// Meta returns a MetaValue that can be passed to CallTool to populate the
// protocol-level _meta object sent with the request.
func Meta(key string, value any) MetaValue {
	return MetaValue{Key: key, Value: value}
}

// MetaForKey returns the value for the given key from a slice of MetaValue, or nil if not found.
// This is pretty inefficient for large slices, but MetaValue is intended for small numbers of values
func MetaForKey(meta []MetaValue, key string) any {
	for _, mv := range meta {
		if mv.Key == key {
			return mv.Value
		}
	}
	return nil
}

// MetaMap converts a slice of MetaValue into a map[string]any
func MetaMap(meta []MetaValue) map[string]any {
	m := make(map[string]any, len(meta))
	for _, mv := range meta {
		m[mv.Key] = mv.Value
	}
	return m
}
