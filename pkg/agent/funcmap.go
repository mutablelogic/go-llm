package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

///////////////////////////////////////////////////////////////////////////////
// TEMPLATE FUNCTIONS

// funcMap returns the custom template functions available in agent templates.
func funcMap() template.FuncMap {
	return template.FuncMap{
		"json": func(v any) (string, error) {
			data, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		"default": func(def, val any) any {
			if val == nil || val == "" {
				return def
			}
			return val
		},
		"join": func(list []any, sep string) string {
			var buf bytes.Buffer
			for i, v := range list {
				if i > 0 {
					buf.WriteString(sep)
				}
				buf.WriteString(fmt.Sprint(v))
			}
			return buf.String()
		},
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"trim":  strings.TrimSpace,
	}
}
