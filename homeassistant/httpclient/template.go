package homeassistant

import (
	"bytes"
	"context"

	// Packages
	"github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// Template renders a Home Assistant Jinja2 template and returns the result
// as plain text. See https://www.home-assistant.io/docs/configuration/templating
func (c *Client) Template(ctx context.Context, template string) (string, error) {
	type reqTemplate struct {
		Template string `json:"template"`
	}

	payload, err := client.NewJSONRequest(reqTemplate{
		Template: template,
	})
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := c.DoWithContext(ctx, payload, &buf, client.OptPath("template")); err != nil {
		return "", err
	}

	return buf.String(), nil
}
