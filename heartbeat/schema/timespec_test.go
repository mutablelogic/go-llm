package schema_test

import (
	"encoding/json"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/heartbeat/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ParseTimeSpec(t *testing.T) {
	for _, test := range []struct {
		in, out string
	}{
		{"0 9 * * 1-5", "0 9 * * 1-5"},
		{"*/15 * * * *", "0,15,30,45 * * * *"},
		{"0-59 * * * *", "* * * * *"},
		{"0 0 1,15 * *", "0 0 1,15 * *"},
		{"30 14 15 6 *", "30 14 15 6 *"},
		{"0 12 * * 7", "0 12 * * 0"},
		{"0 12 * * 0-7", "0 12 * * *"},
		{"0 12 * * 5,7", "0 12 * * 0,5"},
		{"5/20 * * * *", "5,25,45 * * * *"},
		{"0 0 29 2 *", "0 0 29 2 *"},
	} {
		t.Run(test.in, func(t *testing.T) {
			ts, err := schema.ParseTimeSpec(test.in)
			require.NoError(t, err)
			assert.Equal(t, test.out, ts.String())
		})
	}
	for _, in := range []string{"", "* * *", "* * * * * *", "60 * * * *", "* 24 * * *", "* * 0 * *", "* * * 13 *", "* * * * 8", "*/0 * * * *", "5-1 * * * *", "x * * * *", "0 0 31 2 *", "0 0 30,31 2 *"} {
		t.Run(in, func(t *testing.T) {
			_, err := schema.ParseTimeSpec(in)
			assert.ErrorIs(t, err, llm.ErrBadParameter)
		})
	}
}

func Test_TimeSpecJSON(t *testing.T) {
	ts, err := schema.ParseTimeSpec("0 9 * * 1-5")
	require.NoError(t, err)
	data, err := json.Marshal(ts)
	require.NoError(t, err)
	assert.Equal(t, `"0 9 * * 1-5"`, string(data))
	var decoded schema.TimeSpec
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, ts, decoded)
	assert.Error(t, json.Unmarshal([]byte(`"0 25 * * *"`), &decoded))
}

func Test_TimeSpecNext(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	newYork, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	utc := func(s string) time.Time {
		value, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return value
	}
	for name, test := range map[string]struct {
		spec string
		loc  *time.Location
		from string
		next string
	}{
		"Matches now":            {"*/15 * * * *", nil, "2026-01-05T10:00:00Z", "2026-01-05T10:00:00Z"},
		"Next quarter":           {"*/15 * * * *", nil, "2026-01-05T10:07:00Z", "2026-01-05T10:15:00Z"},
		"Seconds round up":       {"* * * * *", nil, "2026-01-05T10:07:30Z", "2026-01-05T10:08:00Z"},
		"Weekday from Friday":    {"0 9 * * 1-5", nil, "2026-01-09T10:00:00Z", "2026-01-12T09:00:00Z"},
		"Next month":             {"0 0 1 6 *", nil, "2026-03-15T00:00:00Z", "2026-06-01T00:00:00Z"},
		"Skips short months":     {"0 0 31 * *", nil, "2026-04-01T00:00:00Z", "2026-05-31T00:00:00Z"},
		"Leap day":               {"0 0 29 2 *", nil, "2026-03-01T00:00:00Z", "2028-02-29T00:00:00Z"},
		"Day or weekday":         {"0 0 13 * 5", nil, "2026-02-01T00:00:00Z", "2026-02-06T00:00:00Z"},
		"Next year":              {"0 0 1 1 *", nil, "2026-06-01T00:00:00Z", "2027-01-01T00:00:00Z"},
		"In a zone":              {"0 12 * * *", newYork, "2026-01-05T00:00:00Z", "2026-01-05T17:00:00Z"},
		"In a zone, in summer":   {"0 12 * * *", newYork, "2026-07-05T00:00:00Z", "2026-07-05T16:00:00Z"},
		"Ahead of UTC":           {"0 9 * * *", berlin, "2026-01-05T09:00:00Z", "2026-01-06T08:00:00Z"},
		"Skipped, clocks go fwd": {"30 2 * * *", berlin, "2026-03-29T00:00:00Z", "2026-03-29T01:30:00Z"},
		"After the skip":         {"30 2 * * *", berlin, "2026-03-29T01:31:00Z", "2026-03-30T00:30:00Z"},
		"Repeated, the first":    {"30 2 * * *", berlin, "2026-10-25T00:00:00Z", "2026-10-25T00:30:00Z"},
		"Repeated, once":         {"30 2 * * *", berlin, "2026-10-25T00:31:00Z", "2026-10-26T01:30:00Z"},
		"Repeated hour, minutes": {"*/30 * * * *", berlin, "2026-10-25T00:31:00Z", "2026-10-25T02:00:00Z"},
	} {
		t.Run(name, func(t *testing.T) {
			ts, err := schema.ParseTimeSpec(test.spec)
			require.NoError(t, err)
			assert.Equal(t, utc(test.next), ts.Next(utc(test.from), test.loc).UTC())
		})
	}
}
