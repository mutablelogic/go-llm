package httpclient_test

import (
	"context"
	"os"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/toolkit/aftership/httpclient"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// Test_Live lists the couriers, and the trackings, and gets one, from
// AfterShip, with the key in AFTERSHIP_API_KEY, or skips, when there's no key,
// or the tests are short. It only reads, so it doesn't change the account, or
// use its quota of trackings, and doesn't log what's tracked, or who for.
func Test_Live(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping a test of the AfterShip API in short mode")
	}
	key := os.Getenv("AFTERSHIP_API_KEY")
	if key == "" {
		t.Skip("AFTERSHIP_API_KEY not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, err := httpclient.New(key)
	require.NoError(t, err)

	t.Run("Couriers", func(t *testing.T) {
		couriers, err := c.ListCouriers(ctx, &httpclient.ListCouriersRequest{Slug: []string{"dhl", "ups"}})
		require.NoError(t, err)
		var slugs []string
		for _, courier := range couriers {
			assert.NotEmpty(t, courier.Name)
			slugs = append(slugs, courier.Slug)
		}
		assert.ElementsMatch(t, []string{"dhl", "ups"}, slugs)
		active, err := c.ListCouriers(ctx, &httpclient.ListCouriersRequest{Active: types.Ptr(true)})
		require.NoError(t, err)
		t.Logf("%d couriers are on", len(active))
	})

	t.Run("Detect", func(t *testing.T) {
		// UPS's own example of a tracking number
		couriers, err := c.DetectCouriers(ctx, &httpclient.DetectCouriersRequest{TrackingNumber: "1Z999AA10123456784"})
		require.NoError(t, err)
		for _, courier := range couriers {
			t.Logf("it could be %s", courier.Slug)
		}
	})

	t.Run("Trackings", func(t *testing.T) {
		list, err := c.ListTrackings(ctx, &httpclient.ListTrackingsRequest{Limit: 2})
		require.NoError(t, err)
		t.Logf("%d trackings, and %d in the page", list.Pagination.Total, len(list.Trackings))
		assert.LessOrEqual(t, len(list.Trackings), 2)
		for _, tracking := range list.Trackings {
			assert.NotEmpty(t, tracking.ID)
			assert.NotEmpty(t, tracking.TrackingNumber)
			assert.NotEmpty(t, tracking.Tag)
			assert.False(t, tracking.CreatedAt.IsZero())
			got, err := c.GetTracking(ctx, tracking.ID, &httpclient.GetTrackingRequest{Lang: "en"})
			require.NoError(t, err)
			assert.Equal(t, tracking.ID, got.ID)
			t.Logf("it's %s, with %s, and has %d checkpoints", got.Tag, got.Slug, len(got.Checkpoints))
		}

		// The next page has others
		if list.Pagination.HasNextPage {
			next, err := c.ListTrackings(ctx, &httpclient.ListTrackingsRequest{Limit: 2, Cursor: list.Pagination.NextCursor})
			require.NoError(t, err)
			require.NotEmpty(t, next.Trackings)
			assert.NotEqual(t, list.Trackings[0].ID, next.Trackings[0].ID)
		}

		// Since a time, which is in the future, so there are none
		none, err := c.ListTrackings(ctx, &httpclient.ListTrackingsRequest{CreatedAtMin: time.Now().Add(24 * time.Hour)})
		if assert.NoError(t, err) {
			assert.Empty(t, none.Trackings)
		}
	})

	t.Run("Errors", func(t *testing.T) {
		_, err := c.GetTracking(ctx, "00000000000000000000000000000000", nil)
		t.Logf("no tracking: %v", err)
		assert.ErrorIs(t, err, llm.ErrNotFound)
		bad, err := httpclient.New("not-a-key")
		require.NoError(t, err)
		_, err = bad.ListTrackings(ctx, nil)
		t.Logf("not a key: %v", err)
		assert.ErrorIs(t, err, llm.ErrForbidden)
	})
}
