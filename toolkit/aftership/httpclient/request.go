package httpclient

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// REQUEST TYPES

// ListTrackingsRequest selects trackings, and a page of them
type ListTrackingsRequest struct {
	Keyword                         string    `json:"keyword,omitempty" help:"Text in the tracking number, title, order, custom fields, or customers' names, emails or phone numbers" optional:""`
	TrackingNumbers                 []string  `json:"tracking_numbers,omitempty" help:"Tracking numbers, of which there can be 50" optional:""`
	Slug                            []string  `json:"slug,omitempty" help:"Codes of couriers, such as dhl" optional:""`
	Tag                             []Tag     `json:"tag,omitempty" help:"Statuses: Pending, InfoReceived, InTransit, OutForDelivery, AttemptFail, Delivered, AvailableForPickup, Exception or Expired" optional:""`
	TransitTime                     *int      `json:"transit_time,omitempty" help:"Days since it was picked up, or until it was delivered" optional:""`
	Origin                          []string  `json:"origin,omitempty" help:"Countries it's from, such as DEU" optional:""`
	Destination                     []string  `json:"destination,omitempty" help:"Countries it's to, such as DEU" optional:""`
	CourierDestinationCountryRegion []string  `json:"courier_destination_country_region,omitempty" help:"Countries the courier says it's to, such as DEU" optional:""`
	CreatedAtMin                    time.Time `json:"created_at_min,omitzero" help:"When it was added, at the earliest, which is up to 120 days ago" optional:""`
	CreatedAtMax                    time.Time `json:"created_at_max,omitzero" help:"When it was added, at the latest" optional:""`
	UpdatedAtMin                    time.Time `json:"updated_at_min,omitzero" help:"When it last changed, at the earliest" optional:""`
	UpdatedAtMax                    time.Time `json:"updated_at_max,omitzero" help:"When it last changed, at the latest" optional:""`
	ReturnToSender                  *bool     `json:"return_to_sender,omitempty" help:"Whether it's returned to its sender" optional:""`
	ShipmentTags                    []string  `json:"shipment_tags,omitempty" help:"Tags of shipments" optional:""`
	OrderID                         []string  `json:"order_id,omitempty" help:"Identifiers of orders" optional:""`
	Fields                          []string  `json:"fields,omitempty" help:"The fields which are returned, of title, order_id, tag and checkpoints, or every field" optional:""`
	Cursor                          string    `json:"cursor,omitempty" help:"The next_cursor of the page before" optional:""`
	Limit                           uint64    `json:"limit,omitempty" help:"Most trackings to return, which is 100 when it isn't given" min:"1" max:"200" optional:""`
}

// GetTrackingRequest is what's returned of a tracking
type GetTrackingRequest struct {
	Fields []string `json:"fields,omitempty" help:"The fields which are returned, such as tag and checkpoints, or every field" optional:""`
	Lang   string   `json:"lang,omitempty" help:"Language which the courier's messages are translated to, such as en" optional:""`
}

// TrackingMeta is what can be set of a tracking, and changed. When it's
// changed, what's nil stays as it was.
type TrackingMeta struct {
	Slug                     *string           `json:"slug,omitempty" help:"Code of the courier, such as dhl, which is detected when it isn't given" optional:""`
	Title                    *string           `json:"title,omitempty" help:"What it's called, which is its tracking number when it isn't given" optional:""`
	Note                     *string           `json:"note,omitempty" help:"A note, or empty, to remove it" optional:""`
	Language                 *string           `json:"language,omitempty" help:"Language of who it's for, such as en" optional:""`
	OrderID                  *string           `json:"order_id,omitempty" help:"Identifier of its order" optional:""`
	OrderIDPath              *string           `json:"order_id_path,omitempty" help:"URL of its order" optional:""`
	OrderNumber              *string           `json:"order_number,omitempty" help:"Number of its order" optional:""`
	OrderDate                *string           `json:"order_date,omitempty" help:"When it was ordered, such as 2026-11-24T11:23:51+01:00" optional:""`
	OrderPromised            *PromisedDate     `json:"order_promised_delivery_date,omitempty" help:"When it's promised" optional:""`
	DeliveryType             *string           `json:"delivery_type,omitempty" enum:"pickup_at_store,pickup_at_courier,door_to_door" help:"How it's delivered" optional:""`
	PickupLocation           *string           `json:"pickup_location,omitempty" help:"Where it's picked up" optional:""`
	PickupNote               *string           `json:"pickup_note,omitempty" help:"A note about where it's picked up" optional:""`
	ShipmentType             *string           `json:"shipment_type,omitempty" help:"The courier's service" optional:""`
	ShipmentDirection        *string           `json:"shipment_direction,omitempty" enum:"forward,return" help:"Whether it's sent, or returned" optional:""`
	ShippingMethod           *string           `json:"shipping_method,omitempty" help:"How it's sent, as it was chosen, such as express" optional:""`
	TrackingAccountNumber    *string           `json:"tracking_account_number,omitempty" help:"The sender's account, which some couriers need" optional:""`
	TrackingKey              *string           `json:"tracking_key,omitempty" help:"A key, which some couriers need" optional:""`
	TrackingShipDate         *string           `json:"tracking_ship_date,omitempty" help:"When it's sent, such as 2026-11-24, which some couriers need" optional:""`
	OriginCountryRegion      *string           `json:"origin_country_region,omitempty" help:"Country it's from, such as DEU" optional:""`
	OriginState              *string           `json:"origin_state,omitempty" help:"State it's from" optional:""`
	OriginCity               *string           `json:"origin_city,omitempty" help:"City it's from" optional:""`
	OriginPostalCode         *string           `json:"origin_postal_code,omitempty" help:"Postal code it's from" optional:""`
	OriginRawLocation        *string           `json:"origin_raw_location,omitempty" help:"Address it's from" optional:""`
	DestinationCountryRegion *string           `json:"destination_country_region,omitempty" help:"Country it's to, such as DEU, which some couriers need" optional:""`
	DestinationState         *string           `json:"destination_state,omitempty" help:"State it's to, which some couriers need" optional:""`
	DestinationCity          *string           `json:"destination_city,omitempty" help:"City it's to" optional:""`
	DestinationPostalCode    *string           `json:"destination_postal_code,omitempty" help:"Postal code it's to, which some couriers need" optional:""`
	DestinationRawLocation   *string           `json:"destination_raw_location,omitempty" help:"Address it's to" optional:""`
	LocationID               *string           `json:"location_id,omitempty" help:"Identifier of the location it's sent from, rather than its address" optional:""`
	CustomFields             map[string]string `json:"custom_fields,omitempty" help:"Other fields, which shouldn't have personal information" optional:""`
	Customers                []Customer        `json:"customers,omitempty" help:"Who it's for, of whom there can be three" optional:""`
}

// CreateTrackingRequest is a tracking to add
type CreateTrackingRequest struct {
	TrackingNumber string `json:"tracking_number" help:"Tracking number, of 4 to 100 characters"`
	TrackingMeta
	ID                  string   `json:"id,omitempty" help:"Identifier of the tracking, which AfterShip gives it when it isn't given" optional:""`
	SlugGroup           string   `json:"slug_group,omitempty" help:"Group of couriers which the courier is detected from, such as fedex-group" optional:""`
	ShipmentTags        []string `json:"shipment_tags,omitempty" help:"Tags of the shipment" optional:""`
	CourierConnectionID string   `json:"courier_connection_id,omitempty" help:"Identifier of the courier's account which it's sent with" optional:""`
	LastMile            *Mile    `json:"last_mile,omitempty" help:"The courier which delivers it, when it's another" optional:""`
}

// UpdateTrackingRequest is what's changed of a tracking
type UpdateTrackingRequest struct {
	TrackingMeta
}

// CompleteTrackingRequest is why a tracking is completed, and when
type CompleteTrackingRequest struct {
	Reason string `json:"reason" enum:"DELIVERED,LOST,RETURNED_TO_SENDER" help:"Why it's completed"`
	Time   string `json:"event_datetime,omitempty" help:"When it was, such as 2026-11-24, or 2026-11-24T11:23:51+01:00" optional:""`
}

// ListCouriersRequest selects couriers
type ListCouriersRequest struct {
	Active *bool    `json:"active,omitempty" help:"Whether only the couriers which are on, in AfterShip, are returned" optional:""`
	Slug   []string `json:"slug,omitempty" help:"Codes of couriers, such as dhl" optional:""`
}

// DetectCouriersRequest is a tracking number, and what else is known of it,
// whose couriers are detected
type DetectCouriersRequest struct {
	TrackingNumber           string   `json:"tracking_number" help:"Tracking number"`
	Slug                     []string `json:"slug,omitempty" help:"Codes of the couriers it could be, such as dhl" optional:""`
	SlugGroup                string   `json:"slug_group,omitempty" help:"Group of couriers it could be, such as fedex-group" optional:""`
	DestinationPostalCode    string   `json:"destination_postal_code,omitempty" help:"Postal code it's to" optional:""`
	DestinationState         string   `json:"destination_state,omitempty" help:"State it's to" optional:""`
	DestinationCountryRegion string   `json:"destination_country_region,omitempty" help:"Country it's to, such as DEU" optional:""`
	OriginCountryRegion      string   `json:"origin_country_region,omitempty" help:"Country it's from, such as DEU" optional:""`
	TrackingShipDate         string   `json:"tracking_ship_date,omitempty" help:"When it's sent, such as 2026-11-24" optional:""`
	TrackingAccountNumber    string   `json:"tracking_account_number,omitempty" help:"The sender's account" optional:""`
	TrackingKey              string   `json:"tracking_key,omitempty" help:"A key, which some couriers need" optional:""`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Values returns the request as URL query parameters
func (r *ListTrackingsRequest) Values() url.Values {
	result := url.Values{}
	if r == nil {
		return result
	}
	set(result, "keyword", r.Keyword)
	set(result, "tracking_numbers", r.TrackingNumbers...)
	set(result, "slug", r.Slug...)
	set(result, "tag", r.Tag...)
	if r.TransitTime != nil {
		result.Set("transit_time", strconv.Itoa(*r.TransitTime))
	}
	set(result, "origin", r.Origin...)
	set(result, "destination", r.Destination...)
	set(result, "courier_destination_country_region", r.CourierDestinationCountryRegion...)
	setTime(result, "created_at_min", r.CreatedAtMin)
	setTime(result, "created_at_max", r.CreatedAtMax)
	setTime(result, "updated_at_min", r.UpdatedAtMin)
	setTime(result, "updated_at_max", r.UpdatedAtMax)
	if r.ReturnToSender != nil {
		result.Set("return_to_sender", strconv.FormatBool(*r.ReturnToSender))
	}
	set(result, "shipment_tags", r.ShipmentTags...)
	set(result, "order_id", r.OrderID...)
	set(result, "fields", r.Fields...)
	set(result, "cursor", r.Cursor)
	if r.Limit > 0 {
		result.Set("limit", strconv.FormatUint(r.Limit, 10))
	}
	return result
}

// Values returns the request as URL query parameters
func (r *GetTrackingRequest) Values() url.Values {
	result := url.Values{}
	if r == nil {
		return result
	}
	set(result, "fields", r.Fields...)
	set(result, "lang", r.Lang)
	return result
}

// Values returns the request as URL query parameters
func (r *ListCouriersRequest) Values() url.Values {
	result := url.Values{}
	if r == nil {
		return result
	}
	if r.Active != nil {
		result.Set("active", strconv.FormatBool(*r.Active))
	}
	set(result, "slug", r.Slug...)
	return result
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// set sets the query parameter to the values which aren't empty, with commas
// between them, when there are any
func set[T ~string](query url.Values, name string, values ...T) {
	var result []string
	for _, value := range values {
		if value := strings.TrimSpace(string(value)); value != "" {
			result = append(result, value)
		}
	}
	if len(result) > 0 {
		query.Set(name, strings.Join(result, ","))
	}
}

// setTime sets the query parameter to the time, unless it's zero
func setTime(query url.Values, name string, value time.Time) {
	if !value.IsZero() {
		query.Set(name, value.UTC().Format(time.RFC3339))
	}
}
