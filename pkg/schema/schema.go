package schema

import "encoding/json"

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func Stringify[T any](v T) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}
