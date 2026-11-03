package httpclient

import (
	"fmt"
	"net/url"
)

////////////////////////////////////////////////////////////////////////////////
// REQUEST TYPES

// CurrentWeatherRequest defines the input for current weather query
type CurrentWeatherRequest struct {
	Query      string `json:"query" help:"Location query (city name, coordinates, IP, etc.)"`
	AirQuality bool   `json:"air_quality,omitempty" help:"Enable air quality data"`
	Pollen     bool   `json:"pollen,omitempty" help:"Enable pollen data"`
	Language   string `json:"language,omitempty" help:"Language code (e.g., 'en', 'fr', 'es')"`
}

// ForecastWeatherRequest defines the input for forecast weather query
type ForecastWeatherRequest struct {
	Query      string `json:"query" help:"Location query (city name, coordinates, IP, etc.)"`
	Days       int    `json:"days" help:"Number of days to forecast (1-14)" min:"1" max:"14"`
	Date       string `json:"date,omitempty" help:"Specific date for forecast (YYYY-MM-DD)"`
	AirQuality bool   `json:"air_quality,omitempty" help:"Enable air quality data"`
	Alerts     bool   `json:"alerts,omitempty" help:"Enable weather alerts"`
	Pollen     bool   `json:"pollen,omitempty" help:"Enable pollen data"`
	Language   string `json:"language,omitempty" help:"Language code (e.g., 'en', 'fr', 'es')"`
}

// AlertsWeatherRequest defines the input for weather alerts query
type AlertsWeatherRequest struct {
	Query    string `json:"query" help:"Location query (city name, coordinates, IP, etc.)"`
	Language string `json:"language,omitempty" help:"Language code (e.g., 'en', 'fr', 'es')"`
}

////////////////////////////////////////////////////////////////////////////////
// METHODS

// Values converts CurrentWeatherRequest to URL query parameters
func (r *CurrentWeatherRequest) Values(apiKey string) url.Values {
	result := url.Values{}
	result.Set("key", apiKey)
	result.Set("q", r.Query)
	if r.AirQuality {
		result.Set("aqi", "yes")
	}
	if r.Pollen {
		result.Set("pollen", "yes")
	}
	if r.Language != "" {
		result.Set("lang", r.Language)
	}
	return result
}

// Values converts ForecastWeatherRequest to URL query parameters
func (r *ForecastWeatherRequest) Values(apiKey string) url.Values {
	result := url.Values{}
	result.Set("key", apiKey)
	result.Set("q", r.Query)
	result.Set("days", fmt.Sprint(r.Days))
	if r.Date != "" {
		result.Set("dt", r.Date)
	}
	if r.AirQuality {
		result.Set("aqi", "yes")
	}
	if r.Alerts {
		result.Set("alerts", "yes")
	}
	if r.Pollen {
		result.Set("pollen", "yes")
	}
	if r.Language != "" {
		result.Set("lang", r.Language)
	}
	return result
}

// Values converts AlertsWeatherRequest to URL query parameters
func (r *AlertsWeatherRequest) Values(apiKey string) url.Values {
	result := url.Values{}
	result.Set("key", apiKey)
	result.Set("q", r.Query)
	if r.Language != "" {
		result.Set("lang", r.Language)
	}
	return result
}
