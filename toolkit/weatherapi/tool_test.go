package weatherapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	weatherapi "github.com/mutablelogic/go-llm/toolkit/weatherapi"
	httpclient "github.com/mutablelogic/go-llm/toolkit/weatherapi/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// newTools returns the tools, with a client for a fake WeatherAPI server,
// and the queries the server received
func newTools(t *testing.T) (map[string]llm.Tool, *[]url.Values) {
	t.Helper()
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		if r.URL.Query().Get("key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Air quality, pollen and alerts are returned when they're requested
		var airQuality, pollen, alerts string
		if r.URL.Query().Get("aqi") == "yes" {
			airQuality = `,"air_quality":{"pm2_5":4.2,"us-epa-index":1}`
		}
		if r.URL.Query().Get("pollen") == "yes" {
			pollen = `,"pollen":{"Grass":12}`
		}
		if r.URL.Query().Get("alerts") == "yes" {
			alerts = `,"alerts":{"alert":[{"headline":"Wind warning","severity":"Moderate"}]}`
		}
		switch r.URL.Path {
		case "/current.json":
			w.Write([]byte(`{"location":{"name":"London","localtime":"2026-10-12 09:00"},"current":{"temp_c":18,"last_updated":"2026-10-12 09:00"` + airQuality + pollen + `}}`))
		case "/forecast.json":
			w.Write([]byte(`{"location":{"name":"London","localtime":"2026-10-12 09:00"},"forecast":{"forecastday":[{"date":"2026-10-12","day":{"maxtemp_c":20` + airQuality + `}}]}` + alerts + `}`))
		case "/alerts.json":
			w.Write([]byte(`{"location":{"name":"London","localtime":"2026-10-12 09:00"},"alerts":{"alert":[{"headline":"Wind warning","severity":"Moderate","desc":"Strong winds"}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	tools, err := toolsOf(weatherapi.New("test-key", client.OptEndpoint(server.URL)))
	require.NoError(t, err)
	result := make(map[string]llm.Tool, len(tools))
	for _, tool := range tools {
		result[tool.Name()] = tool
	}
	return result, &queries
}

func TestNew(t *testing.T) {
	t.Run("MissingKey", func(t *testing.T) {
		tools, err := toolsOf(weatherapi.New(""))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.Nil(t, tools)
	})

	t.Run("Tools", func(t *testing.T) {
		tools, _ := newTools(t)
		assert.Len(t, tools, 3)
		for _, name := range []string{"current", "forecast", "alerts"} {
			tool, ok := tools[name]
			require.True(t, ok, name)
			assert.NotEmpty(t, tool.Description(), name)
			assert.Contains(t, tool.InputSchema().Properties, "query", name)
			assert.Nil(t, tool.OutputSchema(), name)
		}
	})

	t.Run("Names", func(t *testing.T) {
		// The tools have valid names
		tools, err := toolsOf(weatherapi.New("test-key"))
		require.NoError(t, err)
		for _, tool := range tools {
			assert.True(t, llm.IsToolName(tool.Name()), tool.Name())
		}
	})

	t.Run("ForecastDays", func(t *testing.T) {
		// The number of days is limited to 1 to 14 in the schema
		tools, _ := newTools(t)
		days := tools["forecast"].InputSchema().Properties["days"]
		require.NotNil(t, days)
		require.NotNil(t, days.Minimum)
		require.NotNil(t, days.Maximum)
		assert.Equal(t, float64(1), *days.Minimum)
		assert.Equal(t, float64(14), *days.Maximum)
	})
}

func TestRun(t *testing.T) {
	ctx := context.Background()

	t.Run("Current", func(t *testing.T) {
		tools, queries := newTools(t)
		result, err := tools["current"].Run(ctx, json.RawMessage(`{"query":"London","language":"fr"}`))
		require.NoError(t, err)
		data, ok := result.(json.RawMessage)
		require.True(t, ok, "result is %T", result)
		var weather httpclient.Weather
		require.NoError(t, json.Unmarshal(data, &weather))
		assert.Equal(t, "London", weather.Query)
		require.NotNil(t, weather.Location)
		assert.Equal(t, "London", weather.Location.Name)
		require.NotNil(t, weather.Current)
		assert.Equal(t, float64(18), weather.Current.TempC)
		require.Len(t, *queries, 1)
		assert.Equal(t, "London", (*queries)[0].Get("q"))
		assert.Equal(t, "fr", (*queries)[0].Get("lang"))
	})

	t.Run("Forecast", func(t *testing.T) {
		tools, queries := newTools(t)
		result, err := tools["forecast"].Run(ctx, json.RawMessage(`{"query":"London","days":3}`))
		require.NoError(t, err)
		data, ok := result.(json.RawMessage)
		require.True(t, ok, "result is %T", result)
		var forecast httpclient.Forecast
		require.NoError(t, json.Unmarshal(data, &forecast))
		require.Len(t, forecast.Forecast.Day, 1)
		assert.Equal(t, "2026-10-12", forecast.Forecast.Day[0].Date)
		require.Len(t, *queries, 1)
		assert.Equal(t, "3", (*queries)[0].Get("days"))
	})

	t.Run("Alerts", func(t *testing.T) {
		// The alerts are got from the alerts endpoint
		tools, queries := newTools(t)
		result, err := tools["alerts"].Run(ctx, json.RawMessage(`{"query":"London"}`))
		require.NoError(t, err)
		var alerts httpclient.WeatherAlerts
		require.NoError(t, json.Unmarshal(result.(json.RawMessage), &alerts))
		require.NotNil(t, alerts.Alerts)
		require.Len(t, alerts.Alerts.Alert, 1)
		assert.Equal(t, "Wind warning", alerts.Alerts.Alert[0].Headline)
		assert.Equal(t, "Moderate", alerts.Alerts.Alert[0].Severity)
		assert.Equal(t, "Strong winds", alerts.Alerts.Alert[0].Description)
		require.Len(t, *queries, 1)
		assert.Equal(t, "London", (*queries)[0].Get("q"))
	})

	t.Run("AirQualityAndPollen", func(t *testing.T) {
		// Air quality and pollen are returned when they're requested
		tools, _ := newTools(t)
		result, err := tools["current"].Run(ctx, json.RawMessage(`{"query":"London","air_quality":true,"pollen":true}`))
		require.NoError(t, err)
		var weather httpclient.Weather
		require.NoError(t, json.Unmarshal(result.(json.RawMessage), &weather))
		require.NotNil(t, weather.Current)
		assert.Equal(t, httpclient.Measures{"pm2_5": 4.2, "us-epa-index": 1}, weather.Current.AirQuality)
		assert.Equal(t, httpclient.Measures{"Grass": 12}, weather.Current.Pollen)

		// Otherwise they're not
		result, err = tools["current"].Run(ctx, json.RawMessage(`{"query":"London"}`))
		require.NoError(t, err)
		weather = httpclient.Weather{}
		require.NoError(t, json.Unmarshal(result.(json.RawMessage), &weather))
		assert.Nil(t, weather.Current.AirQuality)
		assert.Nil(t, weather.Current.Pollen)
	})

	t.Run("ForecastAlerts", func(t *testing.T) {
		// A forecast has its alerts and air quality when they're requested
		tools, _ := newTools(t)
		result, err := tools["forecast"].Run(ctx, json.RawMessage(`{"query":"London","days":1,"alerts":true,"air_quality":true}`))
		require.NoError(t, err)
		var forecast httpclient.Forecast
		require.NoError(t, json.Unmarshal(result.(json.RawMessage), &forecast))
		require.NotNil(t, forecast.Alerts)
		require.Len(t, forecast.Alerts.Alert, 1)
		assert.Equal(t, "Wind warning", forecast.Alerts.Alert[0].Headline)
		require.Len(t, forecast.Forecast.Day, 1)
		assert.Equal(t, httpclient.Measures{"pm2_5": 4.2, "us-epa-index": 1}, forecast.Forecast.Day[0].Day.AirQuality)
	})

	t.Run("Invalid", func(t *testing.T) {
		tools, queries := newTools(t)
		for name, test := range map[string]struct {
			tool  string
			input string
		}{
			"MissingQuery": {"current", `{}`},
			"NoInput":      {"alerts", ``},
			"InvalidJSON":  {"current", `{`},
			"TooManyDays":  {"forecast", `{"query":"London","days":20}`},
			"NoDays":       {"forecast", `{"query":"London"}`},
		} {
			result, err := tools[test.tool].Run(ctx, json.RawMessage(test.input))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
			assert.Nil(t, result, name)
		}

		// Invalid input isn't sent to WeatherAPI
		assert.Empty(t, *queries)
	})

	t.Run("Cancelled", func(t *testing.T) {
		// The request is cancelled with the context
		tools, queries := newTools(t)
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := tools["current"].Run(ctx, json.RawMessage(`{"query":"London"}`))
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, *queries)
	})
}

// toolsOf returns the tools of the namespace, or nil and the error
func toolsOf(ns toolkit.Namespace, err error) ([]llm.Tool, error) {
	if err != nil {
		return nil, err
	}
	return ns.Tools(), nil
}
