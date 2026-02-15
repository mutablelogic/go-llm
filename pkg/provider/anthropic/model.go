package anthropic

import (
	"context"
	"net/url"
	"regexp"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListModels returns all available models from the Anthropic API
func (c *Client) ListModels(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
	return c.ModelCache.ListModels(ctx, opts, func(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
		var response listModelsResponse

		// Request with pagination
		request := url.Values{}
		result := make([]schema.Model, 0, 100)
		for {
			if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models"), client.OptQuery(request)); err != nil {
				return nil, err
			}

			// Convert to schema.Model
			for _, m := range response.Data {
				result = append(result, m.toSchema())
			}

			// If there are no more models, return
			if !response.HasMore {
				break
			}
			request.Set("after_id", response.LastId)
		}

		// Return models
		return result, nil
	})
}

// GetModel returns a specific model by name or ID
func (c *Client) GetModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	return c.ModelCache.GetModel(ctx, name, func(ctx context.Context, name string) (*schema.Model, error) {
		var response model
		if err := c.DoWithContext(ctx, nil, &response, client.OptPath("models", name)); err != nil {
			return nil, err
		}
		return types.Ptr(response.toSchema()), nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// toSchema converts an API model response to schema.Model
func (m model) toSchema() schema.Model {
	variant, version, date := parseModelId(m.Id)

	meta := make(map[string]any)
	if variant != "" {
		meta["variant"] = variant
	}
	if version != "" {
		meta["version"] = version
	}
	if date != "" {
		meta["date"] = date
	}

	return schema.Model{
		Name:        m.Id,
		Description: m.DisplayName,
		Created:     m.CreatedAt,
		OwnedBy:     schema.Anthropic,
		Meta:        meta,
	}
}

var (
	// Old format: claude-3-5-haiku-20241022 → version=3.5, variant=haiku
	reOldMinor = regexp.MustCompile(`^claude-(\d+)-(\d+)-([a-z]+)-(\d{8})$`)
	// Old format: claude-3-haiku-20240307 → version=3, variant=haiku
	reOldMajor = regexp.MustCompile(`^claude-(\d+)-([a-z]+)-(\d{8})$`)
	// New format: claude-opus-4-5-20251101 → variant=opus, version=4.5
	reNewMinorDate = regexp.MustCompile(`^claude-([a-z]+)-(\d+)-(\d+)-(\d{8})$`)
	// New format: claude-opus-4-20250514 → variant=opus, version=4
	reNewMajorDate = regexp.MustCompile(`^claude-([a-z]+)-(\d+)-(\d{8})$`)
	// New format (no date): claude-opus-4-6 → variant=opus, version=4.6
	reNewMinorOnly = regexp.MustCompile(`^claude-([a-z]+)-(\d+)-(\d{1,2})$`)
)

// parseModelId extracts variant, version and date from a model ID.
func parseModelId(id string) (variant, version, date string) {
	// Old format with minor version: claude-3-5-haiku-20241022
	if parts := reOldMinor.FindStringSubmatch(id); len(parts) == 5 {
		return parts[3], parts[1] + "." + parts[2], parts[4]
	}
	// Old format major only: claude-3-haiku-20240307
	if parts := reOldMajor.FindStringSubmatch(id); len(parts) == 4 {
		return parts[2], parts[1], parts[3]
	}
	// New format with minor version and date: claude-opus-4-5-20251101
	if parts := reNewMinorDate.FindStringSubmatch(id); len(parts) == 5 {
		return parts[1], parts[2] + "." + parts[3], parts[4]
	}
	// New format major only with date: claude-opus-4-20250514
	if parts := reNewMajorDate.FindStringSubmatch(id); len(parts) == 4 {
		return parts[1], parts[2], parts[3]
	}
	// New format with minor version, no date: claude-opus-4-6
	if parts := reNewMinorOnly.FindStringSubmatch(id); len(parts) == 4 {
		return parts[1], parts[2] + "." + parts[3], ""
	}
	return "", "", ""
}
