package httpclient

import (
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The statuses of a tracking, and of a checkpoint
const (
	TagPending            Tag = "Pending"            // added, without information yet
	TagInfoReceived       Tag = "InfoReceived"       // the courier has been told, and hasn't it yet
	TagInTransit          Tag = "InTransit"          // on its way
	TagOutForDelivery     Tag = "OutForDelivery"     // about to be delivered
	TagAttemptFail        Tag = "AttemptFail"        // couldn't be delivered, and will usually be tried again
	TagDelivered          Tag = "Delivered"          // delivered
	TagAvailableForPickup Tag = "AvailableForPickup" // can be picked up
	TagException          Tag = "Exception"          // something's wrong, such as customs, or it's returned
	TagExpired            Tag = "Expired"            // without information for some time
)

// Why a tracking is marked as completed
const (
	ReasonDelivered        = "DELIVERED"
	ReasonLost             = "LOST"
	ReasonReturnedToSender = "RETURNED_TO_SENDER"
)

////////////////////////////////////////////////////////////////////////////////
// RESPONSE TYPES

// Tag is the status of a tracking, or of a checkpoint
type Tag string

// Tracking is a shipment which is tracked. A time which the courier gives is
// text, as it can be a date, or a time without a time zone.
type Tracking struct {
	ID             string    `json:"id"`
	LegacyID       string    `json:"legacy_id,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitzero"`
	UpdatedAt      time.Time `json:"updated_at,omitzero"`
	TrackingNumber string    `json:"tracking_number"`
	Slug           string    `json:"slug,omitempty"` // the courier's code
	Title          string    `json:"title,omitempty"`
	Note           string    `json:"note,omitempty"`
	Language       string    `json:"language,omitempty"`
	Source         string    `json:"source,omitempty"`
	Active         bool      `json:"active"` // whether it's still tracked
	TrackedCount   float64   `json:"tracked_count,omitempty"`

	// Its status, and what's happened
	Tag                    Tag          `json:"tag,omitempty"`
	Subtag                 string       `json:"subtag,omitempty"`
	SubtagMessage          string       `json:"subtag_message,omitempty"`
	Checkpoints            []Checkpoint `json:"checkpoints,omitempty"`
	ReturnToSender         bool         `json:"return_to_sender,omitempty"`
	FailedDeliveryAttempts int          `json:"failed_delivery_attempts,omitempty"`
	SignedBy               string       `json:"signed_by,omitempty"`
	SignatureRequirement   string       `json:"signature_requirement,omitempty"`
	ProofOfDelivery        []Proof      `json:"proof_of_delivery,omitempty"`

	// When it's shipped, and delivered
	TransitTime         *int            `json:"transit_time,omitempty"` // days, or nil before it's picked up
	TrackingShipDate    string          `json:"tracking_ship_date,omitempty"`
	ShipmentPickupDate  string          `json:"shipment_pickup_date,omitempty"`
	FirstAttemptedAt    string          `json:"first_attempted_at,omitempty"`
	ShipmentDelivery    string          `json:"shipment_delivery_date,omitempty"`
	OnTimeStatus        string          `json:"on_time_status,omitempty"`
	OnTimeDifference    float64         `json:"on_time_difference,omitempty"` // days
	FirstEstimated      *EstimatedTime  `json:"first_estimated_delivery,omitempty"`
	LatestEstimated     *EstimatedTime  `json:"latest_estimated_delivery,omitempty"`
	CustomEstimated     *EstimatedTime  `json:"custom_estimated_delivery_date,omitempty"`
	CourierEstimated    *EstimatedDate  `json:"courier_estimated_delivery_date,omitempty"`
	AfterShipEstimated  *EstimatedDate  `json:"aftership_estimated_delivery_date,omitempty"`
	OrderPromised       *PromisedDate   `json:"order_promised_delivery_date,omitempty"`
	DeliveryType        string          `json:"delivery_type,omitempty"`
	DeliveryLocation    string          `json:"delivery_location_type,omitempty"`
	PickupLocation      string          `json:"pickup_location,omitempty"`
	PickupNote          string          `json:"pickup_note,omitempty"`
	ShippingMethod      string          `json:"shipping_method,omitempty"`
	ShipmentDirection   string          `json:"shipment_direction,omitempty"` // forward, or return
	ReturnShipment      *LinkedShipment `json:"return_shipment,omitempty"`
	ForwardShipment     *LinkedShipment `json:"forward_shipment,omitempty"`
	FirstMile           *Mile           `json:"first_mile,omitempty"`
	LastMile            *Mile           `json:"last_mile,omitempty"`
	CourierTrackingLink string          `json:"courier_tracking_link,omitempty"`
	CourierRedirectLink string          `json:"courier_redirect_link,omitempty"`
	TrackingURL         string          `json:"aftership_tracking_url,omitempty"`
	OrderTrackingURL    string          `json:"aftership_tracking_order_url,omitempty"`
	MultiPiece          *MultiPiece     `json:"multi_piece_info,omitempty"`
	Type                string          `json:"shipment_type,omitempty"` // the courier's service
	PackageCount        float64         `json:"shipment_package_count,omitempty"`
	Weight              *Quantity       `json:"shipment_weight,omitempty"`
	Dimensions          *Dimensions     `json:"shipment_dimensions,omitempty"`
	CarbonEmissions     *Quantity       `json:"carbon_emissions,omitempty"`
	ShipmentTags        []string        `json:"shipment_tags,omitempty"`

	// Where it's from, and to
	OriginCountryRegion             string `json:"origin_country_region,omitempty"`
	OriginState                     string `json:"origin_state,omitempty"`
	OriginCity                      string `json:"origin_city,omitempty"`
	OriginPostalCode                string `json:"origin_postal_code,omitempty"`
	OriginRawLocation               string `json:"origin_raw_location,omitempty"`
	DestinationCountryRegion        string `json:"destination_country_region,omitempty"`
	DestinationState                string `json:"destination_state,omitempty"`
	DestinationCity                 string `json:"destination_city,omitempty"`
	DestinationPostalCode           string `json:"destination_postal_code,omitempty"`
	DestinationRawLocation          string `json:"destination_raw_location,omitempty"`
	CourierDestinationCountryRegion string `json:"courier_destination_country_region,omitempty"`
	LocationID                      string `json:"location_id,omitempty"`

	// Its order, and who it's for
	OrderID          string            `json:"order_id,omitempty"`
	OrderIDPath      string            `json:"order_id_path,omitempty"`
	OrderNumber      string            `json:"order_number,omitempty"`
	OrderDate        string            `json:"order_date,omitempty"`
	OrderTags        []string          `json:"order_tags,omitempty"`
	CustomFields     map[string]string `json:"custom_fields,omitempty"`
	Customers        []Customer        `json:"customers,omitempty"`
	SubscribedEmails []string          `json:"subscribed_emails,omitempty"`
	SubscribedSmses  []string          `json:"subscribed_smses,omitempty"`

	// What the courier needs to track it
	TrackingAccountNumber string `json:"tracking_account_number,omitempty"`
	TrackingKey           string `json:"tracking_key,omitempty"`
	CourierConnectionID   string `json:"courier_connection_id,omitempty"`
}

// Checkpoint is something which happened to a shipment, where, and when
type Checkpoint struct {
	CreatedAt         time.Time   `json:"created_at,omitzero"`
	Time              string      `json:"checkpoint_time,omitempty"`
	Slug              string      `json:"slug,omitempty"`
	Tag               Tag         `json:"tag,omitempty"`
	Subtag            string      `json:"subtag,omitempty"`
	SubtagMessage     string      `json:"subtag_message,omitempty"`
	RawTag            string      `json:"raw_tag,omitempty"`
	Message           string      `json:"message,omitempty"`
	Location          string      `json:"location,omitempty"`
	City              string      `json:"city,omitempty"`
	State             string      `json:"state,omitempty"`
	PostalCode        string      `json:"postal_code,omitempty"`
	CountryRegion     string      `json:"country_region,omitempty"`
	CountryRegionName string      `json:"country_region_name,omitempty"`
	Coordinate        *Coordinate `json:"coordinate,omitempty"`
	Events            []Event     `json:"events,omitempty"`
	Source            string      `json:"source,omitempty"` // carrier, or user
	Hash              string      `json:"hash,omitempty"`   // identifies it
}

// Coordinate is where a checkpoint is
type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Event is what a checkpoint says happened, such as that it's returned, and
// why
type Event struct {
	Code   string `json:"code,omitempty"`
	Reason *struct {
		Code string `json:"code,omitempty"`
	} `json:"reason,omitempty"`
}

// EstimatedTime is when a shipment is estimated to be delivered, or between,
// and who estimates it, such as the courier
type EstimatedTime struct {
	Type         string `json:"type,omitempty"` // specific, or range
	Source       string `json:"source,omitempty"`
	Datetime     string `json:"datetime,omitempty"`
	DatetimeMin  string `json:"datetime_min,omitempty"`
	DatetimeMax  string `json:"datetime_max,omitempty"`
	ReviseReason string `json:"revise_reason,omitempty"`
}

// EstimatedDate is the day a shipment is estimated to be delivered, or
// between
type EstimatedDate struct {
	Date           string  `json:"estimated_delivery_date,omitempty"`
	DateMin        string  `json:"estimated_delivery_date_min,omitempty"`
	DateMax        string  `json:"estimated_delivery_date_max,omitempty"`
	ConfidenceCode float64 `json:"confidence_code,omitempty"`
}

// PromisedDate is the day a shipment is promised to be delivered, or between
type PromisedDate struct {
	Date    string `json:"promised_delivery_date,omitempty" help:"When it's promised, such as 2026-11-30"`
	DateMin string `json:"promised_delivery_date_min,omitempty" help:"The earliest it's promised"`
	DateMax string `json:"promised_delivery_date_max,omitempty" help:"The latest it's promised"`
}

// Mile is the first, or last, part of a shipment, which can be another
// courier's
type Mile struct {
	TrackingNumber      string `json:"tracking_number,omitempty" help:"Tracking number of the courier"`
	Slug                string `json:"slug,omitempty" help:"Code of the courier"`
	TransitTime         *int   `json:"transit_time,omitempty" readonly:""`
	CourierTrackingLink string `json:"courier_tracking_link,omitempty" readonly:""`
	CourierRedirectLink string `json:"courier_redirect_link,omitempty" readonly:""`
	Source              string `json:"source,omitempty" readonly:""`
}

// Customer is who a shipment is for, or from
type Customer struct {
	ID          string `json:"id,omitempty"`
	Role        string `json:"role,omitempty" help:"Such as buyer, or receiver"`
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	Language    string `json:"language,omitempty"`
}

// LinkedShipment is the return of a shipment, or the shipment of a return
type LinkedShipment struct {
	ID             string `json:"id,omitempty"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Slug           string `json:"slug,omitempty"`
}

// Proof is proof of delivery, such as a photo, or a signature
type Proof struct {
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

// MultiPiece is the packages of a shipment which has more than one
type MultiPiece struct {
	Type   string `json:"type,omitempty"`
	Pieces []struct {
		TrackingID     string `json:"tracking_id,omitempty"`
		TrackingNumber string `json:"tracking_number,omitempty"`
		Type           string `json:"type,omitempty"`
		Trackable      bool   `json:"trackable,omitempty"`
	} `json:"pieces,omitempty"`
}

// Quantity is a value, and its unit, such as a weight
type Quantity struct {
	Unit  string  `json:"unit,omitempty"`
	Value float64 `json:"value"`
}

// Dimensions are the size of a shipment
type Dimensions struct {
	Unit   string  `json:"unit,omitempty"`
	Length float64 `json:"length,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

// TrackingList is a page of trackings, and how many there are
type TrackingList struct {
	Pagination Pagination `json:"pagination"`
	Trackings  []Tracking `json:"trackings"`
}

// Pagination is how many there are, and the cursor of the next page, when
// there is one
type Pagination struct {
	Total       int    `json:"total"`
	NextCursor  string `json:"next_cursor,omitempty"`
	HasNextPage bool   `json:"has_next_page,omitempty"`
}

// Courier is a courier, by its code, which is its slug, with what a tracking
// of it needs besides its number
type Courier struct {
	Slug                      string   `json:"slug"`
	Name                      string   `json:"name"`
	OtherName                 string   `json:"other_name,omitempty"`
	Phone                     string   `json:"phone,omitempty"`
	WebURL                    string   `json:"web_url,omitempty"`
	DefaultLanguage           string   `json:"default_language,omitempty"`
	SupportLanguages          []string `json:"support_languages,omitempty"`
	ServiceFromCountryRegions []string `json:"service_from_country_regions,omitempty"`
	RequiredFields            []string `json:"required_fields,omitempty"`
	OptionalFields            []string `json:"optional_fields,omitempty"`

	// What a connection to the person's account with it needs, when it needs
	// one
	Credentials *Credentials `json:"credentials,omitempty"`
}

// Credentials are what a connection to an account with a courier needs
type Credentials struct {
	Fields []struct {
		Name     string `json:"name,omitempty"`
		Type     string `json:"type,omitempty"`
		Required bool   `json:"required,omitempty"`
	} `json:"fields,omitempty"`
}

// courierList is the couriers, and how many there are
type courierList struct {
	Total    int       `json:"total"`
	Couriers []Courier `json:"couriers"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (t Tracking) String() string {
	return types.Stringify(t)
}

func (c Checkpoint) String() string {
	return types.Stringify(c)
}

func (l TrackingList) String() string {
	return types.Stringify(l)
}

func (c Courier) String() string {
	return types.Stringify(c)
}
