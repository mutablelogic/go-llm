package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	yaml "gopkg.in/yaml.v3"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	yamlSeparator = "---"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Read parses a markdown file with YAML front matter and returns an AgentMeta.
// The YAML front matter is delimited by "---" lines. The remaining markdown
// body becomes the Template (Go template for the user message).
func Read(r io.Reader) (schema.AgentMeta, error) {
	front, body, err := splitFrontMatter(r)
	if err != nil {
		return schema.AgentMeta{}, err
	}

	var meta schema.AgentMeta
	if len(front) > 0 {
		if err := yaml.Unmarshal(front, &meta); err != nil {
			return schema.AgentMeta{}, llm.ErrBadParameter.Withf("yaml: %v", err)
		}
	}

	// If no name in front matter, extract from reader name
	if meta.Name == "" {
		meta.Name = extractName(r)
	}

	// Validate name is a non-empty identifier
	if !types.IsIdentifier(meta.Name) {
		return schema.AgentMeta{}, llm.ErrBadParameter.Withf("name: must be a non-empty identifier, got %q", meta.Name)
	}

	// If no title in front matter, extract from first markdown heading
	if strings.TrimSpace(meta.Title) == "" {
		meta.Title = extractTitle(body)
	}

	// Validate title is non-empty and at least 10 characters
	if title := strings.TrimSpace(meta.Title); len(title) < 10 {
		return schema.AgentMeta{}, llm.ErrBadParameter.Withf("title: must be at least 10 characters, got %q", meta.Title)
	}

	// Validate format schema
	if err := validateJSONSchema(meta.Format, "format"); err != nil {
		return schema.AgentMeta{}, err
	}

	// Validate input schema
	if err := validateJSONSchema(meta.Input, "input"); err != nil {
		return schema.AgentMeta{}, err
	}

	// Set the template from the markdown body
	meta.Template = strings.TrimSpace(string(body))

	return meta, nil
}

// ReadFile parses a markdown file at the given path and returns an AgentMeta.
func ReadFile(path string) (schema.AgentMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return schema.AgentMeta{}, llm.ErrNotFound.Withf("%v", err)
	}
	defer f.Close()
	return Read(f)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// splitFrontMatter splits a reader into YAML front matter and the remaining
// markdown body. The front matter must be enclosed between two "---" lines
// at the start of the document.
func splitFrontMatter(r io.Reader) ([]byte, []byte, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // support lines up to 1MB

	// Look for opening "---"
	if !scanner.Scan() {
		return nil, nil, nil // empty document
	}
	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != yamlSeparator {
		// No front matter — the entire document is body
		var body bytes.Buffer
		body.WriteString(scanner.Text())
		body.WriteByte('\n')
		for scanner.Scan() {
			body.WriteString(scanner.Text())
			body.WriteByte('\n')
		}
		return nil, body.Bytes(), scanner.Err()
	}

	// Read YAML until closing "---"
	var front bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == yamlSeparator {
			// Found closing separator — rest is body
			var body bytes.Buffer
			for scanner.Scan() {
				body.WriteString(scanner.Text())
				body.WriteByte('\n')
			}
			return front.Bytes(), body.Bytes(), scanner.Err()
		}
		front.WriteString(line)
		front.WriteByte('\n')
	}

	// Reached EOF without closing separator — treat everything as YAML
	return front.Bytes(), nil, scanner.Err()
}

// extractTitle scans the markdown body for the first ATX heading (# Title)
// and returns the title text, or empty string if none found.
func extractTitle(body []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			if title := strings.TrimSpace(strings.TrimPrefix(line, "#")); title != "" {
				return title
			}
		}
	}
	return ""
}

// extractName returns a name derived from the reader if it implements
// a Name() string method (e.g. *os.File). The name is the base filename
// without extension. Returns empty string if the reader has no name.
func extractName(r io.Reader) string {
	type named interface {
		Name() string
	}
	if n, ok := r.(named); ok {
		base := filepath.Base(n.Name())
		ext := filepath.Ext(base)
		return strings.TrimSuffix(base, ext)
	}
	return ""
}

// validateJSONSchema validates a JSONSchema value using google/jsonschema-go.
// Returns nil if the schema is empty (field not present).
func validateJSONSchema(v schema.JSONSchema, field string) error {
	if len(v) == 0 {
		return nil
	}

	// Validate by unmarshalling into jsonschema.Schema
	var s jsonschema.Schema
	if err := json.Unmarshal(v, &s); err != nil {
		return llm.ErrBadParameter.Withf("%s: invalid JSON schema: %v", field, err)
	}

	// Ensure the schema has a type
	if s.Type == "" {
		return llm.ErrBadParameter.Withf("%s: missing required \"type\" field", field)
	}

	return nil
}
