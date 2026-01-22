package weatherapi

import (
	"fmt"
	"net/url"

	// Packages
	"github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

type options struct {
	url.Values
}

type Opt func(*options) error

////////////////////////////////////////////////////////////////////////////////

// Number of days of weather forecast. Value ranges from 1 to 10
func OptDays(days int) Opt {
	return func(o *options) error {
		if days < 1 {
			return llm.ErrBadParameter.With("OptDays")
		}
		o.Set("days", fmt.Sprint(days))
		return nil
	}
}

// Get air quality data
func OptAirQuality() Opt {
	return func(o *options) error {
		o.Set("aqi", "yes")
		return nil
	}
}

// Get weather alert data
func OptAlerts() Opt {
	return func(o *options) error {
		o.Set("alerts", "yes")
		return nil
	}
}
