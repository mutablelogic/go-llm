package httpclient

import (
	"strconv"
	"time"
	_ "time/tzdata"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// RESPONSE TYPES

type Location struct {
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Country        string  `json:"country"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	Timezone       string  `json:"tz_id"`
	LocaltimeEpoch int64   `json:"localtime_epoch"`
	Localtime      Time    `json:"localtime"`
}

type CurrentConditions struct {
	LastUpdatedEpoch int64 `json:"last_updated_epoch"`
	LastUpdated      Time  `json:"last_updated"`
	Conditions
}

type ForecastConditions struct {
	TimeEpoch int64 `json:"time_epoch"`
	Time      Time  `json:"time"`
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
	WindMph    float64  `json:"wind_mph"`
	WindKph    float64  `json:"wind_kph"`
	WindDegree int      `json:"wind_degree"`
	WindDir    string   `json:"wind_dir"`
	PressureMb float64  `json:"pressure_mb"`
	PressureIn float64  `json:"pressure_in"`
	PrecipMm   float64  `json:"precip_mm"`
	PrecipIn   float64  `json:"precip_in"`
	Humidity   int      `json:"humidity"`
	Cloud      int      `json:"cloud"`
	FeelslikeC float64  `json:"feelslike_c"`
	FeelslikeF float64  `json:"feelslike_f"`
	VisKm      float64  `json:"vis_km"`
	VisMiles   float64  `json:"vis_miles"`
	Uv         float64  `json:"uv"`
	GustMph    float64  `json:"gust_mph"`
	GustKph    float64  `json:"gust_kph"`
	AirQuality Measures `json:"air_quality,omitempty"` // Returned when air quality is requested
	Pollen     Measures `json:"pollen,omitempty"`      // Returned when pollen is requested
}

// Measures are measurements by name, such as the air quality's "pm2_5" or
// "us-epa-index", or the pollen's "Grass"
type Measures map[string]float64

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
	AirQuality Measures `json:"air_quality,omitempty"` // Returned when air quality is requested
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
	} `json:"forecast"`
	Alerts *AlertList `json:"alerts,omitempty"` // Returned when alerts are requested
}

// WeatherAlerts are the weather alerts for a location
type WeatherAlerts struct {
	Query    string     `json:"q,omitempty"`
	Location *Location  `json:"location,omitempty"`
	Alerts   *AlertList `json:"alerts,omitempty"`
}

// AlertList is a list of weather alerts
type AlertList struct {
	Alert []*Alert `json:"alert"`
}

// Alert is a weather alert or warning issued by a government agency
type Alert struct {
	Headline    string `json:"headline,omitempty"`
	MsgType     string `json:"msgtype,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Urgency     string `json:"urgency,omitempty"`
	Areas       string `json:"areas,omitempty"`
	Category    string `json:"category,omitempty"`
	Certainty   string `json:"certainty,omitempty"`
	Event       string `json:"event,omitempty"`
	Note        string `json:"note,omitempty"`
	Effective   string `json:"effective,omitempty"`
	Expires     string `json:"expires,omitempty"`
	Description string `json:"desc,omitempty"`
	Instruction string `json:"instruction,omitempty"`
}

type Time struct {
	time.Time
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (w Weather) String() string {
	return types.Stringify(w)
}

func (f Forecast) String() string {
	return types.Stringify(f)
}

func (a WeatherAlerts) String() string {
	return types.Stringify(a)
}

////////////////////////////////////////////////////////////////////////////////
// MARSHAL TIME

// UnmarshalJSON reads a time in WeatherAPI's format, such as
// "2026-10-12 09:00", as UTC until it's set in the location's timezone, or in
// RFC 3339 format, which a time is written as, so a response can be read back
func (t *Time) UnmarshalJSON(data []byte) error {
	unquoted, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	if v, err := time.ParseInLocation("2006-01-02 15:04", unquoted, time.UTC); err == nil {
		t.Time = v
	} else if v, err := time.Parse(time.RFC3339, unquoted); err == nil {
		t.Time = v
	} else {
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// TIMEZONE

// localize sets the times in the weather in the location's timezone
func (w *Weather) localize() {
	if loc := w.Location.location(); loc != nil {
		w.Location.localize(loc)
		w.Current.localize(loc)
	}
}

// localize sets the times in the forecast in the location's timezone
func (f *Forecast) localize() {
	if loc := f.Location.location(); loc != nil {
		f.Location.localize(loc)
		f.Current.localize(loc)
		for _, day := range f.Forecast.Day {
			if day == nil {
				continue
			}
			for _, hour := range day.Hour {
				if hour != nil {
					hour.Time.localize(hour.TimeEpoch, loc)
				}
			}
		}
	}
}

// localize sets the times in the alerts in the location's timezone
func (a *WeatherAlerts) localize() {
	if loc := a.Location.location(); loc != nil {
		a.Location.localize(loc)
	}
}

// location returns the location's timezone, or nil if it's not known
func (l *Location) location() *time.Location {
	if l == nil || l.Timezone == "" {
		return nil
	}
	loc, err := time.LoadLocation(l.Timezone)
	if err != nil {
		return nil
	}
	return loc
}

func (l *Location) localize(loc *time.Location) {
	l.Localtime.localize(l.LocaltimeEpoch, loc)
}

func (c *CurrentConditions) localize(loc *time.Location) {
	if c != nil {
		c.LastUpdated.localize(c.LastUpdatedEpoch, loc)
	}
}

// localize sets the time in the timezone, from its epoch when it's known, as
// it's exact, or otherwise from its wall-clock time
func (t *Time) localize(epoch int64, loc *time.Location) {
	switch {
	case epoch > 0:
		t.Time = time.Unix(epoch, 0).In(loc)
	case !t.IsZero():
		t.Time = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
	}
}
