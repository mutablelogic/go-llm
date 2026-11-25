package httpclient_test

import (
	"context"
	"os"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/toolkit/openstreetmap/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// Test_Live finds well-known places, with the Nominatim server in
// NOMINATIM_URL, or skips, when there's none, or the tests are short. The
// public server's usage policy has to be followed, so it makes few requests,
// one a second.
func Test_Live(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping a test of a Nominatim server in short mode")
	}
	endpoint := os.Getenv("NOMINATIM_URL")
	if endpoint == "" {
		t.Skip("NOMINATIM_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, err := httpclient.New(endpoint)
	require.NoError(t, err)

	// Searched for, and looked up again
	places, err := c.Search(ctx, &httpclient.SearchRequest{Query: "Brandenburger Tor, Berlin", Language: "en", Limit: 1})
	require.NoError(t, err)
	require.Len(t, places, 1)
	place := places[0]
	assert.InDelta(t, 52.516, float64(place.Lat), 0.01)
	assert.InDelta(t, 13.378, float64(place.Lon), 0.01)
	assert.Equal(t, "de", place.Address.CountryCode())
	assert.Contains(t, place.Licence, "OpenStreetMap")
	again, err := c.Lookup(ctx, &httpclient.LookupRequest{Refs: []string{place.Ref()}, Language: "en"})
	require.NoError(t, err)
	require.Len(t, again, 1)
	assert.Equal(t, place.Ref(), again[0].Ref())

	// The city which it's in, and none, in the sea
	city, err := c.Reverse(ctx, &httpclient.ReverseRequest{Lat: 52.516, Lon: 13.378, Zoom: httpclient.ZoomCity, Language: "en"})
	require.NoError(t, err)
	assert.Equal(t, "Berlin", city.Address.City())
	assert.Equal(t, "Germany", city.Address.Country())
	_, err = c.Reverse(ctx, &httpclient.ReverseRequest{Lat: 0, Lon: -30, Zoom: httpclient.ZoomCity})
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
