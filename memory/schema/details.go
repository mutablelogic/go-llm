package schema

import (
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Field is a detail which the memory keeps, about the person who's being
// talked to, or the assistant, such as their name
type Field string

// Detail is what's known of a field: its value, or that the person declined
// to say it, so it isn't in the system prompt. A field which isn't known,
// as it was never said, or was forgotten, has no detail.
type Detail struct {
	Value    string `json:"value,omitempty" help:"The detail, such as a name"`
	Declined bool   `json:"declined,omitempty" help:"The person declined to say it, so it isn't in the system prompt"`
}

// Details are the details which the memory keeps, each of which is nil when
// it isn't known
type Details struct {
	Name      *Detail `json:"name,omitempty" help:"The person's name"`
	Assistant *Detail `json:"assistant,omitempty" help:"The assistant's name"`
	Location  *Detail `json:"location,omitempty" help:"Where the person is, such as their city"`
	Timezone  *Detail `json:"timezone,omitempty" help:"The person's time zone, by its IANA name, such as Europe/Berlin"`
	Language  *Detail `json:"language,omitempty" help:"The language the person speaks"`
}

// DetailRow is a field, and its detail, which is nil when it isn't known, as
// a row of a table
type DetailRow struct {
	Field  Field
	Detail *Detail
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	FieldName      Field = "name"
	FieldAssistant Field = "assistant"
	FieldLocation  Field = "location"
	FieldTimezone  Field = "timezone"
	FieldLanguage  Field = "language"
)

// Fields are the fields, in the order they're described
var Fields = []Field{FieldName, FieldAssistant, FieldLocation, FieldTimezone, FieldLanguage}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// ParseField returns the field with the name, such as location, or
// ErrBadParameter when there's none
func ParseField(name string) (Field, error) {
	field := Field(strings.ToLower(strings.TrimSpace(name)))
	if !field.Valid() {
		return "", llm.ErrBadParameter.Withf("unknown detail %q, which isn't one of %s", name, strings.Join(fieldNames(), ", "))
	}
	return field, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - FIELD

// Valid returns whether the field is one of the fields
func (f Field) Valid() bool {
	for _, field := range Fields {
		if f == field {
			return true
		}
	}
	return false
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - DETAIL

// Validate returns the detail of the field, with its value trimmed, or
// ErrBadParameter when it has a value and is declined, or has neither, or
// its value isn't valid for the field, such as a time zone which isn't known
func (d Detail) Validate(field Field) (Detail, error) {
	d.Value = strings.TrimSpace(d.Value)
	switch {
	case !field.Valid():
		return Detail{}, llm.ErrBadParameter.Withf("unknown detail %q", field)
	case d.Declined && d.Value != "":
		return Detail{}, llm.ErrBadParameter.Withf("%s is declined, so it can't have a value", field)
	case d.Declined:
		return d, nil
	case d.Value == "":
		return Detail{}, llm.ErrBadParameter.Withf("%s has no value, and isn't declined", field)
	}
	if field == FieldTimezone {
		if _, err := time.LoadLocation(d.Value); err != nil || d.Value == "Local" {
			return Detail{}, llm.ErrBadParameter.Withf("unknown time zone %q, which must be an IANA name, such as Europe/Berlin", d.Value)
		}
	}
	return d, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - DETAILS

// Get returns the detail of the field, or nil when it isn't known
func (d Details) Get(field Field) *Detail {
	if p := d.field(field); p != nil && *p != nil {
		detail := **p
		return &detail
	}
	return nil
}

// Set sets the detail of the field, which forgets it, when it's nil, so it
// isn't known. Returns ErrBadParameter when the field, or the detail, isn't
// valid.
func (d *Details) Set(field Field, detail *Detail) error {
	p := d.field(field)
	if p == nil {
		return llm.ErrBadParameter.Withf("unknown detail %q", field)
	} else if detail == nil {
		*p = nil
		return nil
	}
	valid, err := detail.Validate(field)
	if err != nil {
		return err
	}
	*p = &valid
	return nil
}

// Value returns the value of the field, and whether it's known, and not
// declined
func (d Details) Value(field Field) (string, bool) {
	if detail := d.Get(field); detail != nil && !detail.Declined {
		return detail.Value, true
	}
	return "", false
}

// Zone returns the person's time zone, or nil when it isn't known, or
// they declined to say it
func (d Details) Zone() *time.Location {
	if value, ok := d.Value(FieldTimezone); ok {
		if location, err := time.LoadLocation(value); err == nil {
			return location
		}
	}
	return nil
}

// Validate returns ErrBadParameter when a detail isn't valid
func (d Details) Validate() error {
	for _, field := range Fields {
		if detail := d.Get(field); detail != nil {
			if _, err := detail.Validate(field); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d Details) String() string {
	return types.Stringify(d)
}

// Rows returns each field, and its detail, in the order of the fields
func (d Details) Rows() []DetailRow {
	rows := make([]DetailRow, 0, len(Fields))
	for _, field := range Fields {
		rows = append(rows, DetailRow{Field: field, Detail: d.Get(field)})
	}
	return rows
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (DetailRow) Header() []string {
	return []string{"Detail", "State", "Value"}
}

func (DetailRow) Width(int) int {
	return 0
}

// Cell returns the field, whether its detail is known, declined, or not
// known, and its value, when it's known
func (r DetailRow) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return string(r.Field)
	case 1:
		switch {
		case r.Detail == nil:
			return "not known"
		case r.Detail.Declined:
			return "declined"
		}
		return "known"
	case 2:
		if r.Detail != nil {
			return r.Detail.Value
		}
	}
	return ""
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// field returns the field's detail, which can be set, or nil when there's no
// such field
func (d *Details) field(field Field) **Detail {
	switch field {
	case FieldName:
		return &d.Name
	case FieldAssistant:
		return &d.Assistant
	case FieldLocation:
		return &d.Location
	case FieldTimezone:
		return &d.Timezone
	case FieldLanguage:
		return &d.Language
	}
	return nil
}

// fieldNames returns the names of the fields
func fieldNames() []string {
	names := make([]string, 0, len(Fields))
	for _, field := range Fields {
		names = append(names, string(field))
	}
	return names
}
