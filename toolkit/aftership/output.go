package aftership

import (
	"strings"
	"time"

	// Packages
	httpclient "github.com/mutablelogic/go-llm/toolkit/aftership/httpclient"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// tracking is a tracking, as a model is told it, in short. Neither it, nor
// its details, have who it's for, their address, how to reach them, or who
// signed for it. What matters most is first, such as what has happened to it,
// as the last of a result which is too long for a model's context is left out.
type tracking struct {
	ID             string       `json:"id"`
	TrackingNumber string       `json:"tracking_number"`
	Title          string       `json:"title,omitempty"`
	Courier        string       `json:"courier,omitempty"`
	Status         string       `json:"status,omitempty"`
	Checkpoints    []checkpoint `json:"checkpoints,omitzero"` // when there's only one
	Message        string       `json:"message,omitempty"`
	Estimated      string       `json:"estimated_delivery,omitempty"`
	Delivered      string       `json:"delivered,omitempty"`
	Updated        string       `json:"updated,omitempty"`

	// When there's only one
	*details
}

// details are the rest of a tracking. Tracking stops some days after a
// shipment last changes, so one which was delivered can still be tracked.
type details struct {
	Stopped        bool      `json:"tracking_stopped,omitempty"`
	Note           string    `json:"note,omitempty"`
	OrderNumber    string    `json:"order_number,omitempty"`
	Created        string    `json:"created,omitempty"`
	Shipped        string    `json:"shipped,omitempty"`
	PickedUp       string    `json:"picked_up,omitempty"`
	FirstAttempted string    `json:"first_attempted,omitempty"`
	FailedAttempts int       `json:"failed_attempts,omitempty"`
	TransitDays    *int      `json:"transit_days,omitempty"`
	Promised       string    `json:"promised_delivery,omitempty"`
	OnTime         string    `json:"on_time,omitempty"`
	Returned       bool      `json:"return_to_sender,omitempty"`
	From           string    `json:"from,omitempty"`
	To             string    `json:"to,omitempty"`
	PickupLocation string    `json:"pickup_location,omitempty"`
	Service        string    `json:"service,omitempty"`
	Packages       float64   `json:"packages,omitempty"`
	LastMile       *lastMile `json:"last_mile,omitempty"`
	Link           string    `json:"link,omitempty"`
}

// lastMile is the courier which delivers a shipment, when it's another
type lastMile struct {
	Courier        string `json:"courier,omitempty"`
	TrackingNumber string `json:"tracking_number,omitempty"`
}

// checkpoint is something which happened to a shipment, where, and when
type checkpoint struct {
	Time     string `json:"time,omitempty"`
	Status   string `json:"status,omitempty"`
	Message  string `json:"message,omitempty"`
	Location string `json:"location,omitempty"`
}

// courier is a courier, as a model is told it, in short, by its code, and
// whether it needs the person's account with it, as it's the courier's own
// API, rather than what it tells anyone
type courier struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	NeedsAccount bool   `json:"needs_account,omitempty"`

	// When there's only one
	*courierDetails
}

// courierDetails are the rest of a courier, such as what a tracking of it
// needs besides its number
type courierDetails struct {
	OtherName      string   `json:"other_name,omitempty"`
	Phone          string   `json:"phone,omitempty"`
	URL            string   `json:"url,omitempty"`
	Countries      []string `json:"countries,omitempty"`
	RequiredFields []string `json:"required_fields,omitempty"`
	OptionalFields []string `json:"optional_fields,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newTracking returns the tracking, in short
func newTracking(t httpclient.Tracking) tracking {
	result := tracking{
		ID:             t.ID,
		TrackingNumber: t.TrackingNumber,
		Title:          t.Title,
		Courier:        t.Slug,
		Status:         string(t.Tag),
		Message:        t.SubtagMessage,
		Delivered:      t.ShipmentDelivery,
		Updated:        formatTime(t.UpdatedAt),
	}
	if result.Title == result.TrackingNumber {
		result.Title = ""
	}
	if result.Delivered == "" {
		result.Estimated = estimated(t)
	}
	return result
}

// newTrackingDetails returns the tracking, in full
func newTrackingDetails(t httpclient.Tracking) tracking {
	result := newTracking(t)
	result.details = &details{
		Stopped:        !t.Active,
		Note:           t.Note,
		OrderNumber:    t.OrderNumber,
		Created:        formatTime(t.CreatedAt),
		Shipped:        t.TrackingShipDate,
		PickedUp:       t.ShipmentPickupDate,
		FirstAttempted: t.FirstAttemptedAt,
		FailedAttempts: t.FailedDeliveryAttempts,
		TransitDays:    t.TransitTime,
		OnTime:         t.OnTimeStatus,
		Returned:       t.ReturnToSender,
		From:           join(t.OriginCity, t.OriginState, t.OriginCountryRegion),
		To:             join(t.DestinationCity, t.DestinationState, t.DestinationCountryRegion),
		PickupLocation: t.PickupLocation,
		Service:        t.Type,
		Packages:       t.PackageCount,
		Link:           t.CourierTrackingLink,
	}
	result.Checkpoints = make([]checkpoint, 0, len(t.Checkpoints))
	if t.OrderPromised != nil {
		result.Promised = between(t.OrderPromised.Date, t.OrderPromised.DateMin, t.OrderPromised.DateMax)
	}
	if t.LastMile != nil && t.LastMile.TrackingNumber != "" {
		result.LastMile = &lastMile{Courier: t.LastMile.Slug, TrackingNumber: t.LastMile.TrackingNumber}
	}
	for _, c := range t.Checkpoints {
		location := c.Location
		if location == "" {
			country := c.CountryRegionName
			if country == "" {
				country = c.CountryRegion
			}
			location = join(c.City, c.State, country)
		}
		result.Checkpoints = append(result.Checkpoints, checkpoint{Time: c.Time, Status: string(c.Tag), Message: c.Message, Location: location})
	}
	return result
}

// newCourier returns the courier, in short
func newCourier(c httpclient.Courier) courier {
	return courier{ID: c.Slug, Name: c.Name, NeedsAccount: needsAccount(c)}
}

// newCourierDetails returns the courier, in full
func newCourierDetails(c httpclient.Courier) courier {
	result := newCourier(c)
	result.courierDetails = &courierDetails{
		OtherName:      c.OtherName,
		Phone:          c.Phone,
		URL:            c.WebURL,
		Countries:      c.ServiceFromCountryRegions,
		RequiredFields: c.RequiredFields,
		OptionalFields: c.OptionalFields,
	}
	return result
}

// needsAccount returns whether the courier needs the person's account with it,
// which AfterShip says, or its code does
func needsAccount(c httpclient.Courier) bool {
	if c.Credentials != nil && len(c.Credentials.Fields) > 0 {
		return true
	}
	for _, suffix := range []string{"-api", "-sftp", "-webhook"} {
		if strings.HasSuffix(c.Slug, suffix) {
			return true
		}
	}
	return false
}

// estimated returns when the shipment is estimated to be delivered, which is
// as it was last, or else as the courier, or AfterShip, estimates it
func estimated(t httpclient.Tracking) string {
	if e := t.LatestEstimated; e != nil {
		if result := between(e.Datetime, e.DatetimeMin, e.DatetimeMax); result != "" {
			return result
		}
	}
	for _, e := range []*httpclient.EstimatedDate{t.CourierEstimated, t.AfterShipEstimated} {
		if e != nil {
			if result := between(e.Date, e.DateMin, e.DateMax); result != "" {
				return result
			}
		}
	}
	return ""
}

// between returns the date, or that it's between the earliest, and the latest
func between(date, earliest, latest string) string {
	if earliest != "" && latest != "" && earliest != latest {
		return earliest + " to " + latest
	} else if date != "" {
		return date
	} else if earliest != "" {
		return earliest
	}
	return latest
}

// join returns the parts of a place which aren't empty
func join(parts ...string) string {
	var result []string
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return strings.Join(result, ", ")
}

// formatTime returns the time, or empty, when it's zero
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// ptr returns the text, or nil, when it's empty, so it isn't set, as a model
// gives empty fields
func ptr(text string) *string {
	if text = strings.TrimSpace(text); text == "" {
		return nil
	}
	return types.Ptr(text)
}
