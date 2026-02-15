package weatherapi

import (
	"strconv"
	"time"

	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// RESPONSE TYPES

type Location struct {
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Country        string  `json:"country"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Timezone       string  `json:"timezone"`
	LocaltimeEpoch int64   `json:"localtime_epoch"`
	Localtime      Time    `json:"localtime,omitempty"`
}

type CurrentConditions struct {
	LastUpdatedEpoch int64 `json:"last_updated_epoch"`
	LastUpdated      Time  `json:"last_updated,omitempty"`
	Conditions
}

type ForecastConditions struct {
	TimeEpoch int64 `json:"time_epoch"`
	Time      Time  `json:"time,omitempty"`
	Conditions
}

type Conditions struct {
	TempC     float64 `json:"temp_c"`
	TempF     float64 `json:"temp_f"`
	IsDay     int     `json:"is_day"` // Whether to show day condition icon (1) or night icon (0)
	Condition struct {
		Text string `json:"text"`
		Icon string `json:"icon"`
		Code int    `json:"code"`
	} `json:"condition"`
	WindMph    float64 `json:"wind_mph"`
	WindKph    float64 `json:"wind_kph"`
	WindDegree int     `json:"wind_degree"`
	WindDir    string  `json:"wind_dir"`
	PressureMb float64 `json:"pressure_mb"`
	PressureIn float64 `json:"pressure_in"`
	PrecipMm   float64 `json:"precip_mm"`
	PrecipIn   float64 `json:"precip_in"`
	Humidity   int     `json:"humidity"`
	Cloud      int     `json:"cloud"`
	FeelslikeC float64 `json:"feelslike_c"`
	FeelslikeF float64 `json:"feelslike_f"`
	VisKm      float64 `json:"vis_km"`
	VisMiles   float64 `json:"vis_miles"`
	Uv         float64 `json:"uv"`
	GustMph    float64 `json:"gust_mph"`
	GustKph    float64 `json:"gust_kph"`
}

type Day struct {
	MaxTempC            float64 `json:"maxtemp_c"`
	MaxTempF            float64 `json:"maxtemp_f"`
	MinTempC            float64 `json:"mintemp_c"`
	MinTempF            float64 `json:"mintemp_f"`
	AvgTempC            float64 `json:"avgtemp_c"`
	AvgTempF            float64 `json:"avgtemp_f"`
	MaxWindMph          float64 `json:"maxwind_mph"`
	MaxWindKph          float64 `json:"maxwind_kph"`
	TotalPrecipMm       float64 `json:"totalprecip_mm"`
	TotalPrecipIn       float64 `json:"totalprecip_in"`
	TotalSnowCm         float64 `json:"totalsnow_cm"`
	AvgVisKm            float64 `json:"avgvis_km"`
	AvgVisMiles         float64 `json:"avgvis_miles"`
	AvgHumidity         int     `json:"avghumidity"`
	WillItRain          int     `json:"daily_will_it_rain"`
	WillItSnow          int     `json:"daily_will_it_snow"`
	ChanceOfRainPercent int     `json:"daily_chance_of_rain"`
	ChanceOfSnowPercent int     `json:"daily_chance_of_snow"`
	Uv                  float32 `json:"uv"`
	Condition           struct {
		Text string `json:"text"`
		Icon string `json:"icon"`
		Code int    `json:"code"`
	} `json:"condition"`
}

type ForecastDay struct {
	Date      string                `json:"date"`
	DateEpoch int64                 `json:"date_epoch"`
	Day       *Day                  `json:"day"`
	Hour      []*ForecastConditions `json:"hour"`
	Astro     *Astro                `json:"astro"`
}

type Astro struct {
	SunRise          string `json:"sunrise"`
	SunSet           string `json:"sunset"`
	MoonRise         string `json:"moonrise"`
	MoonSet          string `json:"moonset"`
	MoonPhase        string `json:"moon_phase"`
	MoonIllumination int    `json:"moon_illumination"`
	IsMoonUp         int    `json:"is_moon_up"`
	IsSunUp          int    `json:"is_sun_up"`
}

type Weather struct {
	Id       int                `json:"custom_id,omitempty"`
	Query    string             `json:"q,omitempty"`
	Location *Location          `json:"location,omitempty"`
	Current  *CurrentConditions `json:"current,omitempty"`
}

type Forecast struct {
	Id       int                `json:"custom_id,omitempty"`
	Query    string             `json:"q,omitempty"`
	Location *Location          `json:"location,omitempty"`
	Current  *CurrentConditions `json:"current,omitempty"`
	Forecast struct {
		Day []*ForecastDay `json:"forecastday"`
	} `json:"forecast,omitempty"`
}

type Time struct {
	time.Time
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (w Weather) String() string {
	return types.Stringify(w)
}

func (f Forecast) String() string {
	return types.Stringify(f)
}

///////////////////////////////////////////////////////////////////////////////
// MARSHAL TIME

func (t *Time) UnmarshalJSON(data []byte) error {
	if unquoted, err := strconv.Unquote(string(data)); err != nil {
		return err
	} else if v, err := time.ParseInLocation("2006-01-02 15:04", unquoted, time.Local); err != nil {
		return err
	} else {
		t.Time = v
	}
	return nil
}
