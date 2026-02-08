package homeassistant

import (
	"context"
	"encoding/json"

	// Packages
	"github.com/mutablelogic/go-client"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Config represents the Home Assistant server configuration.
type Config struct {
	Components    []string       `json:"components"`
	ConfigDir     string         `json:"config_dir"`
	Elevation     float64        `json:"elevation"`
	Latitude      float64        `json:"latitude"`
	Longitude     float64        `json:"longitude"`
	LocationName  string         `json:"location_name"`
	TimeZone      string         `json:"time_zone"`
	UnitSystem    map[string]any `json:"unit_system"`
	Version       string         `json:"version"`
	ExternalDirs  []string       `json:"whitelist_external_dirs,omitempty"`
	AllowlistDirs []string       `json:"allowlist_external_dirs,omitempty"`
	AllowlistURLs []string       `json:"allowlist_external_urls,omitempty"`
	Currency      string         `json:"currency,omitempty"`
	Country       string         `json:"country,omitempty"`
	Language      string         `json:"language,omitempty"`
	SafeMode      bool           `json:"safe_mode,omitempty"`
	State         string         `json:"state,omitempty"`
	InternalURL   string         `json:"internal_url,omitempty"`
	ExternalURL   string         `json:"external_url,omitempty"`
	RecoveryMode  bool           `json:"recovery_mode,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// Config returns the current server configuration.
func (c *Client) Config(ctx context.Context) (*Config, error) {
	var response Config
	if err := c.DoWithContext(ctx, nil, &response, client.OptPath("config")); err != nil {
		return nil, err
	}
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (v Config) String() string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}
