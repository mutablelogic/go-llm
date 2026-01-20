package weatherapi

import (
	"context"

	// Packages
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// CURRENT WEATHER

type current_weather struct {
	*Client `json:"-"`
	City    string `name:"city" help:"City name" required:"true"`
}

func (current_weather) Name() string {
	return "current_weather"
}

func (current_weather) Description() string {
	return "Return the current weather for a city"
}

func (weather current_weather) Run(ctx context.Context) (any, error) {
	if weather.City == "" {
		return nil, nil
	}
	return weather.Current(weather.City)

}

var _ llm.Tool = (*current_weather)(nil)

///////////////////////////////////////////////////////////////////////////////
// FORECAST WEATHER

type forecast_weather struct {
	*Client `json:"-"`
	City    string `name:"city" help:"City name" required:"true"`
	Days    uint   `name:"days" help:"Number of days to forecast ahead" required:"true"`
}

func (forecast_weather) Name() string {
	return "forecast_weather"
}

func (forecast_weather) Description() string {
	return "Return the current weather for a city"
}

func (weather forecast_weather) Run(ctx context.Context) (any, error) {
	if weather.City == "" {
		return nil, nil
	}
	return weather.Forecast(weather.City, OptDays(int(weather.Days)))

}

var _ llm.Tool = (*forecast_weather)(nil)
