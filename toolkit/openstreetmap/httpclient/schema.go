package httpclient

import (
	"encoding/json"
	"strconv"
	"strings"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// RESPONSE TYPES

// Place is a place of OpenStreetMap
type Place struct {
	OSMType     string            `json:"osm_type,omitempty" help:"Type of its OpenStreetMap object: node, way or relation, or none, such as of a postal code"`
	OSMID       Int               `json:"osm_id,omitempty" help:"Identifier of its OpenStreetMap object"`
	Lat         Float             `json:"lat" help:"Latitude of its centre"`
	Lon         Float             `json:"lon" help:"Longitude of its centre"`
	Category    string            `json:"category,omitempty" help:"Key of its main tag" example:"place"`
	Type        string            `json:"type,omitempty" help:"Value of its main tag" example:"city"`
	AddressType string            `json:"addresstype,omitempty" help:"What part of an address it is" example:"city"`
	Rank        Int               `json:"place_rank,omitempty" help:"How detailed it is, up to 30"`
	Importance  Float             `json:"importance,omitempty" help:"How well known it is, from 0 to 1"`
	Name        string            `json:"name,omitempty" help:"Its name"`
	DisplayName string            `json:"display_name,omitempty" help:"Its full address, separated by commas"`
	Address     Address           `json:"address,omitempty" help:"The parts of its address"`
	BoundingBox BoundingBox       `json:"boundingbox,omitempty" help:"Its area: lowest, and highest, latitude, and lowest, and highest, longitude"`
	ExtraTags   map[string]string `json:"extratags,omitempty" help:"More of what's known, such as its website"`
	Licence     string            `json:"licence,omitempty" help:"Who the data is of, which has to be said where it's shown"`
}

// Address is the parts of an address, by what they are, such as city, or road.
// They differ from place to place, and country to country.
type Address map[string]string

// BoundingBox is the lowest, and highest, latitude, and the lowest, and
// highest, longitude, of an area
type BoundingBox []Float

// Float is a number, which can be returned as a string
type Float float64

// Int is a number, which can be returned as a string
type Int int64

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The parts of an address, the likeliest first, which are its city, and its
// district, whatever they're called where it is
var (
	cityParts     = []string{"city", "town", "village", "municipality", "hamlet"}
	districtParts = []string{"borough", "city_district", "district", "suburb", "quarter", "neighbourhood"}
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Ref returns the place's OpenStreetMap object, such as R62422, which it's
// looked up by, or empty, when it has none
func (p Place) Ref() string {
	switch p.OSMType {
	case "node", "way", "relation":
		if p.OSMID > 0 {
			return strings.ToUpper(p.OSMType[:1]) + strconv.FormatInt(int64(p.OSMID), 10)
		}
	}
	return ""
}

// City returns the city, town or village, or empty, when there's none
func (a Address) City() string { return a.first(cityParts) }

// District returns the part of the city, or empty, when there's none
func (a Address) District() string { return a.first(districtParts) }

// State returns the state, or empty, when there's none
func (a Address) State() string { return a["state"] }

// Country returns the country, or empty, when there's none, such as in the sea
func (a Address) Country() string { return a["country"] }

// CountryCode returns the country's ISO 3166-1 alpha-2 code, in lower case,
// such as de, or empty, when there's none
func (a Address) CountryCode() string { return a["country_code"] }

func (p Place) String() string {
	return types.Stringify(p)
}

// UnmarshalJSON reads a number, or a string of one, or null, or an empty
// string, as zero
func (f *Float) UnmarshalJSON(data []byte) error {
	text, err := numberText(data)
	if err != nil || text == "" {
		*f = 0
		return err
	}
	value, err := strconv.ParseFloat(text, 64)
	*f = Float(value)
	return err
}

// UnmarshalJSON reads a number, or a string of one, or null, or an empty
// string, as zero
func (i *Int) UnmarshalJSON(data []byte) error {
	text, err := numberText(data)
	if err != nil || text == "" {
		*i = 0
		return err
	}
	value, err := strconv.ParseInt(text, 10, 64)
	*i = Int(value)
	return err
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// first returns the first of the parts which the address has
func (a Address) first(parts []string) string {
	for _, part := range parts {
		if value := a[part]; value != "" {
			return value
		}
	}
	return ""
}

// numberText returns the number, which can be a string, as text, or empty,
// when it's null
func numberText(data []byte) (string, error) {
	if string(data) == "null" {
		return "", nil
	}
	if len(data) > 0 && data[0] == '"' {
		var text string
		err := json.Unmarshal(data, &text)
		return strings.TrimSpace(text), err
	}
	return string(data), nil
}
