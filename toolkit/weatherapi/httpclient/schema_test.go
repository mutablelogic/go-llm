package httpclient

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	// Packages
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

////////////////////////////////////////////////////////////////////////////////
// TESTS: Timezone

func Test_Weather_Localize(t *testing.T) {
	// 11:15 in London, which is 10:15 UTC in British Summer Time
	utc := time.Date(2026, 10, 12, 10, 15, 0, 0, time.UTC)
	london, err := time.LoadLocation("Europe/London")
	require.NoError(t, err)

	t.Run("Epoch", func(t *testing.T) {
		var weather Weather
		require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{
			"location": {"name":"London","tz_id":"Europe/London","localtime":"2026-10-12 11:15","localtime_epoch":%d},
			"current": {"last_updated":"2026-10-12 11:00","last_updated_epoch":%d}
		}`, utc.Unix(), utc.Add(-15*time.Minute).Unix())), &weather))
		weather.localize()

		assert.Equal(t, "Europe/London", weather.Location.Timezone)
		assert.True(t, utc.Equal(weather.Location.Localtime.Time), "localtime %v", weather.Location.Localtime)
		assert.Equal(t, london, weather.Location.Localtime.Location())
		assert.Equal(t, 11, weather.Location.Localtime.Hour())
		assert.True(t, utc.Add(-15*time.Minute).Equal(weather.Current.LastUpdated.Time), "last_updated %v", weather.Current.LastUpdated)

		// The time is written with London's offset
		data, err := json.Marshal(weather.Location.Localtime)
		require.NoError(t, err)
		assert.Equal(t, `"2026-10-12T11:15:00+01:00"`, string(data))
	})

	t.Run("WallClock", func(t *testing.T) {
		// Without an epoch, the wall-clock time is in the location's timezone
		var weather Weather
		require.NoError(t, json.Unmarshal([]byte(`{
			"location": {"name":"London","tz_id":"Europe/London","localtime":"2026-10-12 11:15"}
		}`), &weather))
		weather.localize()
		assert.True(t, utc.Equal(weather.Location.Localtime.Time), "localtime %v", weather.Location.Localtime)
	})

	t.Run("UnknownTimezone", func(t *testing.T) {
		// Without a known timezone, the wall-clock time is left in UTC
		var weather Weather
		require.NoError(t, json.Unmarshal([]byte(`{
			"location": {"name":"Nowhere","tz_id":"Nowhere/Else","localtime":"2026-10-12 11:15"}
		}`), &weather))
		weather.localize()
		assert.Equal(t, time.Date(2026, 10, 12, 11, 15, 0, 0, time.UTC), weather.Location.Localtime.Time)
	})

	t.Run("Forecast", func(t *testing.T) {
		// The hours of the forecast are in the location's timezone
		var forecast Forecast
		require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{
			"location": {"name":"London","tz_id":"Europe/London","localtime":"2026-10-12 11:15","localtime_epoch":%d},
			"forecast": {"forecastday":[{"date":"2026-10-12","hour":[{"time":"2026-10-12 11:00","time_epoch":%d},{"time":"2026-10-12 12:00"}]}]}
		}`, utc.Unix(), utc.Add(-15*time.Minute).Unix())), &forecast))
		forecast.localize()
		hours := forecast.Forecast.Day[0].Hour
		assert.True(t, utc.Add(-15*time.Minute).Equal(hours[0].Time.Time), "hour %v", hours[0].Time)
		assert.True(t, utc.Add(45*time.Minute).Equal(hours[1].Time.Time), "hour %v", hours[1].Time)
		assert.Equal(t, 12, hours[1].Time.In(london).Hour())
	})
}
