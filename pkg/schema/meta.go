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

// MetaForKey returns the last value for the given key from a slice of MetaValue,
// or nil if not found. Returning the last match is consistent with MetaMap, where
// later entries override earlier ones.
func MetaForKey(meta []MetaValue, key string) any {
	var result any
	for _, mv := range meta {
		if mv.Key == key {
			result = mv.Value
		}
	}
	return result
}

// MetaMap converts a slice of MetaValue into a map[string]any
func MetaMap(meta []MetaValue) map[string]any {
	m := make(map[string]any, len(meta))
	for _, mv := range meta {
		m[mv.Key] = mv.Value
	}
	return m
}
