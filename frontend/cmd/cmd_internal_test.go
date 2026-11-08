//go:build !client

package cmd

import (
	"net/url"
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func Test_PublicEndpoint(t *testing.T) {
	// The server's own URL is its public endpoint, when it's https, as it is
	// when it was given its public name, and isn't otherwise, as it's then
	// its listen address
	assert.Equal(t, "https://llm.example.com/api", publicEndpoint(&url.URL{Scheme: "https", Host: "llm.example.com", Path: "/api/"}))
	assert.Equal(t, "https://llm.example.com", publicEndpoint(&url.URL{Scheme: "https", Host: "llm.example.com", Path: "/"}))
	assert.Empty(t, publicEndpoint(&url.URL{Scheme: "http", Host: "0.0.0.0:8086", Path: "/api"}))
	assert.Empty(t, publicEndpoint(&url.URL{Scheme: "https"}))
	assert.Empty(t, publicEndpoint(nil))
}
