package prompt

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	yaml "gopkg.in/yaml.v3"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// meta holds all parsed front-matter fields. It is stored as a named (non-
// embedded) field in prompt so that the Name/Title/Description field names
// don't clash with the llm.Prompt interface methods of the same names.
type meta struct {
	schema.GeneratorMeta `yaml:",inline"`

	// Prompt identity
	Name        string            `yaml:"name"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Template    string            `yaml:"-"`
	Input       schema.JSONSchema `yaml:"input"`
	Tools       []string          `yaml:"tools"`
}

// prompt is the private implementation of llm.Prompt parsed from a markdown
// file with optional YAML front matter.
type prompt struct {
	m meta
}

var _ llm.Prompt = (*prompt)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var reH1 = regexp.MustCompile(`(?m)^# (.+)$`)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READ

// Read parses a markdown file with optional YAML front matter from r and
// returns an llm.Prompt. The name is taken from the front matter or derived
// from the reader's filename. The template is set from the markdown body.
func Read(r io.Reader) (llm.Prompt, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	p := new(prompt)
	content := string(data)
	if after, found := strings.CutPrefix(content, "---\n"); found {
		if before, body, ok := strings.Cut(after, "\n---"); ok {
			if err := yaml.Unmarshal([]byte(before), &p.m); err != nil {
				return nil, llm.ErrBadParameter.Withf("yaml: %v", err)
			}
			p.m.Template = strings.TrimSpace(strings.TrimPrefix(body, "\n"))
		}
	} else {
		p.m.Template = strings.TrimSpace(content)
	}
	if p.m.Name == "" {
		p.m.Name = extractName(r)
	}
	if p.m.Title == "" {
		p.m.Title = extractH1(p.m.Template)
	}
	if !types.IsIdentifier(p.m.Name) {
		return nil, llm.ErrBadParameter.Withf("name: must be a non-empty identifier, got %q", p.m.Name)
	}
	if err := validateJSONSchema(p.m.Input); err != nil {
		return nil, llm.ErrBadParameter.Withf("input: %v", err)
	}
	if err := validateJSONSchema(schema.JSONSchema(p.m.Format)); err != nil {
		return nil, llm.ErrBadParameter.Withf("output: %v", err)
	}

	// Return the prompt with the parsed metadata and template
	return p, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - PROMPT

func (p *prompt) Name() string {
	return p.m.Name
}

func (p *prompt) Title() string {
	return p.m.Title
}

func (p *prompt) Description() string {
	return p.m.Description
}

func (p *prompt) MarshalJSON() ([]byte, error) {
	type promptJSON struct {
		Name        string           `json:"name"`
		Title       string           `json:"title,omitempty"`
		Description string           `json:"description,omitempty"`
		Arguments   []promptArgument `json:"arguments,omitempty"`
	}
	return json.Marshal(promptJSON{
		Name:        p.m.Name,
		Title:       p.m.Title,
		Description: p.m.Description,
		Arguments:   argsFromInput(p.m.Input),
	})
}

func (p *prompt) String() string {
	return types.Stringify(p)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// validateJSONSchema returns an error if the schema bytes are non-empty but
// not a valid JSON schema with a "type" field.
func validateJSONSchema(v schema.JSONSchema) error {
	if len(v) == 0 {
		return nil
	} else if s, err := jsonschema.FromJSON(json.RawMessage(v)); err != nil {
		return err
	} else if s.Type == "" {
		return errors.New("missing required \"type\" field")
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE TYPES

// promptArgument represents a single argument in the MCP wire format.
type promptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func extractName(r io.Reader) string {
	type named interface{ Name() string }
	if n, ok := r.(named); ok {
		base := filepath.Base(n.Name())
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return ""
}

// extractH1 returns the text of the first "# Heading" line in the template,
// or an empty string if none is found.
func extractH1(template string) string {
	if m := reH1.FindStringSubmatch(template); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// argsFromInput derives prompt arguments from a JSON Schema's top-level
// properties, matching the MCP wire format. Properties are sorted alphabetically.
func argsFromInput(s schema.JSONSchema) []promptArgument {
	if len(s) == 0 {
		return nil
	}
	type schemaProperty struct {
		Description string `json:"description"`
		Title       string `json:"title"`
	}
	type schemaDoc struct {
		Properties map[string]schemaProperty `json:"properties"`
		Required   []string                  `json:"required"`
	}
	var doc schemaDoc
	if err := json.Unmarshal(s, &doc); err != nil || len(doc.Properties) == 0 {
		return nil
	}
	required := make(map[string]bool, len(doc.Required))
	for _, r := range doc.Required {
		required[r] = true
	}
	names := make([]string, 0, len(doc.Properties))
	for name := range doc.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	args := make([]promptArgument, 0, len(names))
	for _, name := range names {
		prop := doc.Properties[name]
		desc := prop.Description
		if desc == "" {
			desc = prop.Title
		}
		args = append(args, promptArgument{
			Name:        name,
			Description: desc,
			Required:    required[name],
		})
	}
	return args
}
