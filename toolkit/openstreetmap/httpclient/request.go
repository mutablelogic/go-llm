package httpclient

import (
	"math"
	"net/url"
	"strconv"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////////
// REQUEST TYPES

// SearchRequest is which places are searched for: those of the query, or of
// the parts of an address, but not both
type SearchRequest struct {
	Query string `json:"query,omitempty" help:"What's searched for, such as a place's name, or an address, rather than its parts"`

	// The parts of an address, in place of the query
	Amenity    string `json:"amenity,omitempty" help:"Name, or type, of a point of interest"`
	Street     string `json:"street,omitempty" help:"House number, and street"`
	City       string `json:"city,omitempty" help:"City"`
	County     string `json:"county,omitempty" help:"County"`
	State      string `json:"state,omitempty" help:"State"`
	Country    string `json:"country,omitempty" help:"Country"`
	PostalCode string `json:"postalcode,omitempty" help:"Postal code"`

	CountryCodes []string `json:"countrycodes,omitempty" help:"Countries the places are in, by their ISO 3166-1 alpha-2 codes, such as de"`
	Layers       []Layer  `json:"layers,omitempty" help:"Themes of the places, or every one"`
	FeatureType  Feature  `json:"feature_type,omitempty" help:"Type of the places, of addresses, such as city, or every one"`
	ViewBox      *Box     `json:"viewbox,omitempty" help:"Area which the places are preferably in"`
	Bounded      bool     `json:"bounded,omitempty" help:"Whether the places are only in the area"`
	Limit        uint     `json:"limit,omitempty" help:"Most places, up to 40, which is 10 when it isn't given"`
	Language     string   `json:"language,omitempty" help:"Language of the names, such as en, or de,en, which is the place's own when it isn't given"`
	ExtraTags    bool     `json:"extratags,omitempty" help:"Whether a place has more of what's known, such as its website, and opening hours"`
}

// ReverseRequest is the coordinates which a place is nearest to
type ReverseRequest struct {
	Lat       float64 `json:"lat" help:"Latitude, from -90 to 90" min:"-90" max:"90"`
	Lon       float64 `json:"lon" help:"Longitude, from -180 to 180" min:"-180" max:"180"`
	Zoom      Zoom    `json:"zoom,omitempty" help:"Detail of the place, such as 10 for a city, up to 18, a building, which it is when it isn't given"`
	Layers    []Layer `json:"layers,omitempty" help:"Themes of the place, or addresses, and points of interest"`
	Language  string  `json:"language,omitempty" help:"Language of the names, such as en, or de,en, which is the place's own when it isn't given"`
	ExtraTags bool    `json:"extratags,omitempty" help:"Whether the place has more of what's known, such as its website, and opening hours"`
}

// LookupRequest is the OpenStreetMap objects whose places are looked up
type LookupRequest struct {
	Refs      []string `json:"refs" help:"OpenStreetMap objects, up to 50, by their type, N, W or R, and identifier, such as R62422"`
	Language  string   `json:"language,omitempty" help:"Language of the names, such as en, or de,en, which is the place's own when it isn't given"`
	ExtraTags bool     `json:"extratags,omitempty" help:"Whether the places have more of what's known, such as their website, and opening hours"`
}

// Box is an area, from one corner to the other
type Box struct {
	MinLat float64 `json:"min_lat" help:"Lowest latitude"`
	MinLon float64 `json:"min_lon" help:"Lowest longitude"`
	MaxLat float64 `json:"max_lat" help:"Highest latitude"`
	MaxLon float64 `json:"max_lon" help:"Highest longitude"`
}

// Layer is a theme of places
type Layer string

// Feature is a type of places, of addresses
type Feature string

// Zoom is the detail of a place, from 1, the least, to 18, the most
type Zoom uint

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	LayerAddress Layer = "address" // Addresses, streets, towns and boundaries
	LayerPOI     Layer = "poi"     // Points of interest, such as shops
	LayerRailway Layer = "railway"
	LayerNatural Layer = "natural" // Rivers, lakes and mountains
	LayerManMade Layer = "manmade" // What isn't of another layer
)

const (
	FeatureCountry    Feature = "country"
	FeatureState      Feature = "state"
	FeatureCity       Feature = "city"
	FeatureSettlement Feature = "settlement" // From a state to a neighbourhood
)

const (
	ZoomCountry       Zoom = 3
	ZoomState         Zoom = 5
	ZoomCounty        Zoom = 8
	ZoomCity          Zoom = 10
	ZoomTown          Zoom = 12 // Or a borough
	ZoomVillage       Zoom = 13 // Or a suburb
	ZoomNeighbourhood Zoom = 14
	ZoomStreet        Zoom = 17
	ZoomBuilding      Zoom = 18
)

const (
	maxLimit = 40 // Most places of a search
	maxRefs  = 50 // Most objects of a lookup
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Values returns the request as query parameters, or ErrBadParameter when
// there's nothing to search for, or a query as well as the parts of an address
func (r *SearchRequest) Values() (url.Values, error) {
	if r == nil {
		return nil, llm.ErrBadParameter.With("missing what's searched for")
	}
	values := newValues(r.Language, r.ExtraTags)
	parts := false
	for name, value := range map[string]string{"amenity": r.Amenity, "street": r.Street, "city": r.City, "county": r.County, "state": r.State, "country": r.Country, "postalcode": r.PostalCode} {
		if value = strings.TrimSpace(value); value != "" {
			values.Set(name, value)
			parts = true
		}
	}
	switch query := strings.TrimSpace(r.Query); {
	case query == "" && !parts:
		return nil, llm.ErrBadParameter.With("missing what's searched for")
	case query != "" && parts:
		return nil, llm.ErrBadParameter.With("a search has a query, or the parts of an address, but not both")
	case query != "":
		values.Set("q", query)
	}
	setList(values, "countrycodes", r.CountryCodes)
	setList(values, "layer", r.Layers)
	if r.FeatureType != "" {
		values.Set("featureType", string(r.FeatureType))
	}
	if r.ViewBox != nil {
		if err := r.ViewBox.validate(); err != nil {
			return nil, err
		}
		values.Set("viewbox", strings.Join([]string{formatFloat(r.ViewBox.MinLon), formatFloat(r.ViewBox.MinLat), formatFloat(r.ViewBox.MaxLon), formatFloat(r.ViewBox.MaxLat)}, ","))
		if r.Bounded {
			values.Set("bounded", "1")
		}
	} else if r.Bounded {
		return nil, llm.ErrBadParameter.With("a search which is bounded needs its area")
	}
	if r.Limit > maxLimit {
		return nil, llm.ErrBadParameter.Withf("a limit of %d is more than %d", r.Limit, maxLimit)
	} else if r.Limit > 0 {
		values.Set("limit", strconv.FormatUint(uint64(r.Limit), 10))
	}
	return values, nil
}

// Values returns the request as query parameters, or ErrBadParameter when its
// coordinates, or zoom, aren't valid
func (r *ReverseRequest) Values() (url.Values, error) {
	if r == nil {
		return nil, llm.ErrBadParameter.With("missing coordinates")
	} else if err := validate(r.Lat, r.Lon); err != nil {
		return nil, err
	} else if r.Zoom > ZoomBuilding {
		return nil, llm.ErrBadParameter.Withf("a zoom of %d is more than %d", r.Zoom, ZoomBuilding)
	}
	values := newValues(r.Language, r.ExtraTags)
	values.Set("lat", formatFloat(r.Lat))
	values.Set("lon", formatFloat(r.Lon))
	if r.Zoom > 0 {
		values.Set("zoom", strconv.FormatUint(uint64(r.Zoom), 10))
	}
	setList(values, "layer", r.Layers)
	return values, nil
}

// Values returns the request as query parameters, or ErrBadParameter when it
// has no objects, too many, or one which isn't valid
func (r *LookupRequest) Values() (url.Values, error) {
	if r == nil || len(r.Refs) == 0 {
		return nil, llm.ErrBadParameter.With("missing OpenStreetMap objects")
	} else if len(r.Refs) > maxRefs {
		return nil, llm.ErrBadParameter.Withf("%d OpenStreetMap objects are more than %d", len(r.Refs), maxRefs)
	}
	refs := make([]string, 0, len(r.Refs))
	for _, ref := range r.Refs {
		upper := strings.ToUpper(strings.TrimSpace(ref))
		if len(upper) < 2 || !strings.ContainsRune("NWR", rune(upper[0])) {
			return nil, llm.ErrBadParameter.Withf("%q isn't an OpenStreetMap object, such as R62422", ref)
		} else if id, err := strconv.ParseUint(upper[1:], 10, 63); err != nil || id == 0 {
			return nil, llm.ErrBadParameter.Withf("%q isn't an OpenStreetMap object, such as R62422", ref)
		}
		refs = append(refs, upper)
	}
	values := newValues(r.Language, r.ExtraTags)
	values.Set("osm_ids", strings.Join(refs, ","))
	return values, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newValues returns the query parameters of every request: its format, with
// the parts of addresses, the language, and whether there's more
func newValues(language string, extraTags bool) url.Values {
	values := url.Values{"format": {"jsonv2"}, "addressdetails": {"1"}}
	if language = strings.TrimSpace(language); language != "" {
		values.Set("accept-language", language)
	}
	if extraTags {
		values.Set("extratags", "1")
	}
	return values
}

// setList sets the parameter to the values which aren't empty, when there are
// any
func setList[T ~string](values url.Values, name string, list []T) {
	var result []string
	for _, value := range list {
		if value := strings.TrimSpace(string(value)); value != "" {
			result = append(result, value)
		}
	}
	if len(result) > 0 {
		values.Set(name, strings.Join(result, ","))
	}
}

// validate returns ErrBadParameter when the coordinates aren't on Earth
func validate(lat, lon float64) error {
	if math.IsNaN(lat) || math.Abs(lat) > 90 {
		return llm.ErrBadParameter.With("the latitude isn't from -90 to 90")
	} else if math.IsNaN(lon) || math.Abs(lon) > 180 {
		return llm.ErrBadParameter.With("the longitude isn't from -180 to 180")
	}
	return nil
}

func (b *Box) validate() error {
	if err := validate(b.MinLat, b.MinLon); err != nil {
		return err
	} else if err := validate(b.MaxLat, b.MaxLon); err != nil {
		return err
	} else if b.MinLat >= b.MaxLat || b.MinLon >= b.MaxLon {
		return llm.ErrBadParameter.With("the area's lowest corner isn't below its highest")
	}
	return nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
