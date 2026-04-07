package weatherapi

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TESTS: CurrentWeatherRequest

func Test_CurrentWeatherRequest_Values(t *testing.T) {
	tests := []struct {
		name   string
		req    *CurrentWeatherRequest
		apiKey string
		expect url.Values
	}{
		{
			name:   "minimal request",
			req:    &CurrentWeatherRequest{Query: "London"},
			apiKey: "test-key",
			expect: url.Values{
				"key": []string{"test-key"},
				"q":   []string{"London"},
			},
		},
		{
			name:   "with air quality enabled",
			req:    &CurrentWeatherRequest{Query: "Berlin", AirQuality: true},
			apiKey: "test-key-2",
			expect: url.Values{
				"key": []string{"test-key-2"},
				"q":   []string{"Berlin"},
				"aqi": []string{"yes"},
			},
		},
		{
			name:   "with custom language",
			req:    &CurrentWeatherRequest{Query: "Paris", Language: "fr"},
			apiKey: "my-api-key",
			expect: url.Values{
				"key":  []string{"my-api-key"},
				"q":    []string{"Paris"},
				"lang": []string{"fr"},
			},
		},
		{
			name:   "all parameters set",
			req:    &CurrentWeatherRequest{Query: "Tokyo", AirQuality: true, Pollen: true, Language: "ja"},
			apiKey: "key-123",
			expect: url.Values{
				"key":    []string{"key-123"},
				"q":      []string{"Tokyo"},
				"aqi":    []string{"yes"},
				"pollen": []string{"yes"},
				"lang":   []string{"ja"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.Values(tt.apiKey)
			assert.Equal(t, tt.expect, got)
		})
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS: ForecastWeatherRequest

func Test_ForecastWeatherRequest_Values(t *testing.T) {
	tests := []struct {
		name   string
		req    *ForecastWeatherRequest
		apiKey string
		expect url.Values
	}{
		{
			name:   "minimal request",
			req:    &ForecastWeatherRequest{Query: "London", Days: 1},
			apiKey: "test-key",
			expect: url.Values{
				"key":  []string{"test-key"},
				"q":    []string{"London"},
				"days": []string{"1"},
			},
		},
		{
			name:   "with air quality and alerts",
			req:    &ForecastWeatherRequest{Query: "Berlin", Days: 3, AirQuality: true, Alerts: true},
			apiKey: "test-key-2",
			expect: url.Values{
				"key":    []string{"test-key-2"},
				"q":      []string{"Berlin"},
				"days":   []string{"3"},
				"aqi":    []string{"yes"},
				"alerts": []string{"yes"},
			},
		},
		{
			name:   "with custom language and date",
			req:    &ForecastWeatherRequest{Query: "Paris", Days: 5, Language: "fr", Date: "2026-02-01"},
			apiKey: "my-api-key",
			expect: url.Values{
				"key":  []string{"my-api-key"},
				"q":    []string{"Paris"},
				"days": []string{"5"},
				"dt":   []string{"2026-02-01"},
				"lang": []string{"fr"},
			},
		},
		{
			name:   "max days with all options",
			req:    &ForecastWeatherRequest{Query: "Tokyo", Days: 10, AirQuality: true, Alerts: true, Pollen: true, Language: "ja"},
			apiKey: "key-123",
			expect: url.Values{
				"key":    []string{"key-123"},
				"q":      []string{"Tokyo"},
				"days":   []string{"10"},
				"aqi":    []string{"yes"},
				"alerts": []string{"yes"},
				"pollen": []string{"yes"},
				"lang":   []string{"ja"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.Values(tt.apiKey)
			assert.Equal(t, tt.expect, got)
		})
	}
}

///////////////////////////////////////////////////////////////////////////////
// TESTS: AlertsWeatherRequest

func Test_AlertsWeatherRequest_Values(t *testing.T) {
	tests := []struct {
		name   string
		req    *AlertsWeatherRequest
		apiKey string
		expect url.Values
	}{
		{
			name:   "minimal request",
			req:    &AlertsWeatherRequest{Query: "London"},
			apiKey: "test-key",
			expect: url.Values{
				"key":    []string{"test-key"},
				"q":      []string{"London"},
				"alerts": []string{"yes"},
			},
		},
		{
			name:   "with custom language",
			req:    &AlertsWeatherRequest{Query: "Berlin", Language: "de"},
			apiKey: "test-key-2",
			expect: url.Values{
				"key":    []string{"test-key-2"},
				"q":      []string{"Berlin"},
				"alerts": []string{"yes"},
				"lang":   []string{"de"},
			},
		},
		{
			name:   "spanish language",
			req:    &AlertsWeatherRequest{Query: "Madrid", Language: "es"},
			apiKey: "my-api-key",
			expect: url.Values{
				"key":    []string{"my-api-key"},
				"q":      []string{"Madrid"},
				"alerts": []string{"yes"},
				"lang":   []string{"es"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.Values(tt.apiKey)
			assert.Equal(t, tt.expect, got)
		})
	}
}
