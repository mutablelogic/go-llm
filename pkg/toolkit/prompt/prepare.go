package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"text/template"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READ

// Prepare renders the prompt's Go template against the variables in input (a
// JSON object) and returns the resulting string along with any opts derived
// from the prompt's front matter (model, provider, system prompt, etc.).
func (p *prompt) Prepare(_ context.Context, input json.RawMessage) (string, []opt.Opt, error) {
	// Parse the template
	tmpl, err := template.New(p.m.Name).Parse(p.m.Template)
	if err != nil {
		return "", nil, llm.ErrBadParameter.Withf("template parse: %v", err)
	}

	// Unmarshal input variables (nil input is treated as an empty object)
	var vars map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &vars); err != nil {
			return "", nil, llm.ErrBadParameter.Withf("input unmarshal: %v", err)
		}
	}

	// Render the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", nil, llm.ErrBadParameter.Withf("template execute: %v", err)
	}

	// Build opts from front matter
	var opts []opt.Opt
	if p.m.Model != "" {
		opts = append(opts, opt.WithModel(p.m.Model))
	}
	if p.m.SystemPrompt != "" {
		opts = append(opts, opt.WithSystemPrompt(p.m.SystemPrompt))
	}

	return buf.String(), opts, nil
}
