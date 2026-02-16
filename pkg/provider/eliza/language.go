package eliza

import (
	"embed"
	"encoding/json"
	"path/filepath"
	"strings"
)

//go:embed lang/*.json
var langFS embed.FS

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Language defines all the text data for an ELIZA language variant
type Language struct {
	Model             string            `json:"model"`
	Description       string            `json:"description"`
	LanguageCode      string            `json:"language"`
	Quits             []string          `json:"quits"`
	Greetings         []string          `json:"greetings"`
	Reflections       map[string]string `json:"reflections"`
	Rules             []Rule            `json:"rules"`
	GreetingResponses []string          `json:"greetingResponses"`
	GoodbyeResponses  []string          `json:"goodbyeResponses"`
	DefaultResponses  []string          `json:"defaultResponses"`
	MemoryResponses   []string          `json:"memoryResponses"`
}

// Rule defines a pattern-matching rule with its responses
type Rule struct {
	Pattern   string   `json:"pattern"`
	Responses []string `json:"responses"`
	Memorable bool     `json:"memorable,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// LoadLanguages reads all embedded lang/*.json files and returns them keyed by model name
func LoadLanguages() (map[string]*Language, error) {
	entries, err := langFS.ReadDir("lang")
	if err != nil {
		return nil, err
	}

	languages := make(map[string]*Language, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := langFS.ReadFile(filepath.Join("lang", entry.Name()))
		if err != nil {
			return nil, err
		}
		var lang Language
		if err := json.Unmarshal(data, &lang); err != nil {
			return nil, err
		}
		languages[lang.Model] = &lang
	}

	return languages, nil
}
