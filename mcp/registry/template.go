package registry

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"net/url"
	"slices"
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	yaml "go.yaml.in/yaml/v3"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// servers are the templates of the remote MCP servers which are offered, as
// connectors which can be added, by default
//
//go:embed servers.yaml
var servers []byte

// TemplateIconPath is the path of a template's logo, among the web UI's
// files, which its icon is the file name of
const TemplateIconPath = "icon/connector/"

// MaxIconSize is the largest data URL of an icon, which a server sends, as
// it's connected, which a connector's status has
const MaxIconSize = 64 * 1024

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Templates returns the templates of the remote MCP servers which are
// offered by default, from servers.yaml, which is embedded
func Templates() ([]schema.ConnectorTemplate, error) {
	return ParseTemplates(servers)
}

// ParseTemplates returns the templates in the YAML, which is a list of
// them, or ErrBadParameter, when it isn't, or one isn't valid, as it has no
// name, or its URL, or credentials' names, aren't valid
func ParseTemplates(data []byte) ([]schema.ConnectorTemplate, error) {
	var result []schema.ConnectorTemplate
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return nil, llm.ErrBadParameter.Withf("templates: %v", err)
	}
	for _, template := range result {
		if err := template.Validate(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListTemplates returns the templates of the connectors which can be added,
// whose credentials the registry's credentials provider had, as it was
// created, and whose server isn't a connector's already, in order of their
// names
func (r *Registry) ListTemplates(ctx context.Context) (*schema.ConnectorTemplateList, error) {
	connectors, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}
	added := make(map[string]bool, len(connectors))
	for _, connector := range connectors {
		added[sameURL(connector.URL)] = true
	}
	list := &schema.ConnectorTemplateList{Body: make([]*schema.ConnectorTemplate, 0, len(r.offered))}
	for _, template := range r.offered {
		if !added[sameURL(template.URL)] {
			list.Body = append(list.Body, &template)
		}
	}
	list.Count = uint(len(list.Body))
	return list, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// sameURL returns the URL as it's compared, to say whether a connector is
// of a template: without the space around it, with its scheme, and host, in
// lower case, and without a slash at the end of its path, as it's escaped,
// so an escaped slash, such as in /tenant%2Fid/mcp, isn't another
// separator, and the URL another's
func sameURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	u.Scheme, u.Host = strings.ToLower(u.Scheme), strings.ToLower(u.Host)
	escaped := strings.TrimSuffix(u.EscapedPath(), "/")
	path, err := url.PathUnescape(escaped)
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	u.Path, u.RawPath = path, escaped
	return u.String()
}

// offer returns the templates whose credentials the credentials provider
// has, so a connector of one can be signed in to, in order of their names
func (r *Registry) offer(ctx context.Context, templates []schema.ConnectorTemplate) []schema.ConnectorTemplate {
	var result []schema.ConnectorTemplate
	for _, template := range templates {
		// A credential which is empty, such as an environment variable which
		// is set to nothing, as Docker Compose sets one which isn't given, isn't
		// had, so its template isn't offered
		id, secret, err := r.clientCredentials(ctx, schema.Connector{ConnectorMeta: template.Meta()})
		if err != nil || (template.ClientIDCredential != "" && strings.TrimSpace(id) == "") || (template.ClientSecretCredential != "" && strings.TrimSpace(secret) == "") {
			continue
		}
		result = append(result, template)
	}
	slices.SortFunc(result, func(a, b schema.ConnectorTemplate) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return result
}

// templateIcon returns the logo of the template of the connector's server,
// by its URL, as a URL of the web UI's, or none, when it isn't a well-known
// server's, or its template has none
func (r *Registry) templateIcon(connector schema.Connector) string {
	url := sameURL(connector.URL)
	for _, template := range r.templates {
		if template.Icon != "" && sameURL(template.URL) == url {
			return TemplateIconPath + template.Icon
		}
	}
	return ""
}

// serverIcon returns the icon the server sent, as it was connected, which
// is a data URL of an image, rather than one elsewhere, which would be
// fetched from the server's domain, preferring one for a light background,
// as the web UI shows it on one, and none, when it sent none of them, or
// they're too large
func serverIcon(icons []sdkmcp.Icon) string {
	var found string
	for _, icon := range icons {
		src := strings.TrimSpace(icon.Source)
		if !strings.HasPrefix(strings.ToLower(src), "data:image/") || len(src) > MaxIconSize {
			continue
		} else if icon.Theme == "" || icon.Theme == "light" {
			return src
		} else if found == "" {
			found = src
		}
	}
	return found
}
