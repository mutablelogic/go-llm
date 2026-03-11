package prompt

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
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
// PUBLIC METHODS

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
// llm.Prompt INTERFACE

func (p *prompt) Name() string        { return p.m.Name }
func (p *prompt) Title() string       { return p.m.Title }
func (p *prompt) Description() string { return p.m.Description }

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
