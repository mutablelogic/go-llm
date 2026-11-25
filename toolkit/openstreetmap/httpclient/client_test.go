package httpclient_test

import (
	"bytes"
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	transport "github.com/mutablelogic/go-client/pkg/transport"
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/toolkit/openstreetmap/httpclient"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

////////////////////////////////////////////////////////////////////////////////
// HELPERS

// server is a fake Nominatim server, which returns the status, and the body,
// and the last request it received, and how many
type server struct {
	*httptest.Server
	status      int
	contentType string
	location    string
	body        string
	path        string
	query       url.Values
	header      http.Header
	count       int
}

func newServer(t *testing.T, opts ...client.ClientOpt) (*server, *httpclient.Client) {
	t.Helper()
	s := &server{status: http.StatusOK, contentType: "application/json; charset=utf-8"}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.path, s.query, s.header = r.URL.Path, r.URL.Query(), r.Header
		s.count++
		w.Header().Set("Content-Type", s.contentType)
		if s.location != "" {
			w.Header().Set("Location", s.location)
		}
		w.WriteHeader(s.status)
		w.Write([]byte(s.body))
	}))
	t.Cleanup(s.Close)
	c, err := httpclient.New(" "+s.URL+"/osm ", append([]client.ClientOpt{client.OptRateLimit(0)}, opts...)...)
	require.NoError(t, err)
	return s, c
}

// berlin is a place, as Nominatim returns it, whose numbers are strings, or
// not, as they are in its documentation, and its code
const berlin = `{"place_id":1,"licence":"Data © OpenStreetMap contributors","osm_type":"relation","osm_id":62422,"lat":"52.5108850","lon":"13.3989367",
	"category":"boundary","type":"administrative","place_rank":"8","importance":0.87,"addresstype":"city","name":"Berlin","display_name":"Berlin, Deutschland",
	"address":{"borough":"Mitte","city":"Berlin","state":"Berlin","ISO3166-2-lvl4":"DE-BE","country":"Deutschland","country_code":"de"},
	"boundingbox":["52.3382448","52.6755087","13.0883450","13.7611609"],"extratags":{"population":"3769962"},"a_new_field":{"x":1}}`

////////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_New(t *testing.T) {
	for _, endpoint := range []string{"", " "} {
		_, err := httpclient.New(endpoint)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	}
	_, err := httpclient.New("not a url")
	assert.Error(t, err)
	c, err := httpclient.New(httpclient.PublicEndpoint)
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func Test_Search(t *testing.T) {
	ctx := t.Context()
	s, c := newServer(t)
	s.body = "[" + berlin + "]"
	places, err := c.Search(ctx, &httpclient.SearchRequest{Query: " Berlin & more ", CountryCodes: []string{"de", " ", "at"}, Layers: []httpclient.Layer{httpclient.LayerAddress, httpclient.LayerPOI},
		FeatureType: httpclient.FeatureCity, ViewBox: &httpclient.Box{MinLat: 52, MinLon: 13, MaxLat: 53, MaxLon: 14.5}, Bounded: true, Limit: 5, Language: "en", ExtraTags: true})
	require.NoError(t, err)
	assert.Equal(t, "/osm/search", s.path)
	assert.Equal(t, url.Values{"format": {"jsonv2"}, "addressdetails": {"1"}, "extratags": {"1"}, "accept-language": {"en"}, "q": {"Berlin & more"}, "countrycodes": {"de,at"}, "layer": {"address,poi"}, "featureType": {"city"}, "viewbox": {"13,52,14.5,53"}, "bounded": {"1"}, "limit": {"5"}}, s.query)

	// What's known of the place
	require.Len(t, places, 1)
	place := places[0]
	assert.Equal(t, "Berlin", place.Name)
	assert.Equal(t, "R62422", place.Ref())
	assert.Equal(t, httpclient.Float(52.510885), place.Lat)
	assert.Equal(t, httpclient.Float(13.3989367), place.Lon)
	assert.Equal(t, httpclient.Int(8), place.Rank)
	assert.Equal(t, httpclient.Float(0.87), place.Importance)
	assert.Equal(t, httpclient.BoundingBox{52.3382448, 52.6755087, 13.088345, 13.7611609}, place.BoundingBox)
	assert.Equal(t, "3769962", place.ExtraTags["population"])
	assert.Equal(t, "Berlin", place.Address.City())
	assert.Equal(t, "Mitte", place.Address.District())
	assert.Equal(t, "Berlin", place.Address.State())
	assert.Equal(t, "Deutschland", place.Address.Country())
	assert.Equal(t, "de", place.Address.CountryCode())

	// The parts of an address, and no places
	s.body = "[]"
	places, err = c.Search(ctx, &httpclient.SearchRequest{Street: "1 Main Street", City: "Springfield", PostalCode: "12345"})
	require.NoError(t, err)
	assert.Empty(t, places)
	assert.Equal(t, url.Values{"format": {"jsonv2"}, "addressdetails": {"1"}, "street": {"1 Main Street"}, "city": {"Springfield"}, "postalcode": {"12345"}}, s.query)

	// Those which aren't valid aren't sent
	count := s.count
	for name, req := range map[string]*httpclient.SearchRequest{
		"nil":          nil,
		"empty":        {Query: " "},
		"both":         {Query: "Berlin", City: "Berlin"},
		"limit":        {Query: "Berlin", Limit: 41},
		"bounded":      {Query: "Berlin", Bounded: true},
		"box":          {Query: "Berlin", ViewBox: &httpclient.Box{MinLat: 53, MinLon: 13, MaxLat: 52, MaxLon: 14}},
		"box latitude": {Query: "Berlin", ViewBox: &httpclient.Box{MinLat: -91, MinLon: 13, MaxLat: 52, MaxLon: 14}},
	} {
		_, err := c.Search(ctx, req)
		assert.ErrorIs(t, err, llm.ErrBadParameter, name)
	}
	assert.Equal(t, count, s.count)
}

func Test_Reverse(t *testing.T) {
	ctx := t.Context()
	s, c := newServer(t)
	s.body = berlin
	place, err := c.Reverse(ctx, &httpclient.ReverseRequest{Lat: 52.52, Lon: 13.405, Zoom: httpclient.ZoomCity, Language: "de", Layers: []httpclient.Layer{httpclient.LayerAddress}})
	require.NoError(t, err)
	assert.Equal(t, "/osm/reverse", s.path)
	assert.Equal(t, url.Values{"format": {"jsonv2"}, "addressdetails": {"1"}, "accept-language": {"de"}, "lat": {"52.52"}, "lon": {"13.405"}, "zoom": {"10"}, "layer": {"address"}}, s.query)
	assert.Equal(t, "Berlin", place.Address.City())

	// Zero is a place, off Africa, where there's none
	s.body = `{"error":"Unable to geocode"}`
	_, err = c.Reverse(ctx, &httpclient.ReverseRequest{})
	assert.ErrorIs(t, err, llm.ErrNotFound)
	assert.Equal(t, "0", s.query.Get("lat"))
	assert.False(t, s.query.Has("zoom"))

	count := s.count
	for name, req := range map[string]*httpclient.ReverseRequest{
		"nil": nil, "latitude": {Lat: 90.1}, "longitude": {Lon: -180.1}, "nan": {Lat: math.NaN()}, "infinite": {Lon: math.Inf(1)}, "zoom": {Zoom: 19},
	} {
		_, err := c.Reverse(ctx, req)
		assert.ErrorIs(t, err, llm.ErrBadParameter, name)
	}
	assert.Equal(t, count, s.count)
}

func Test_Lookup(t *testing.T) {
	ctx := t.Context()
	s, c := newServer(t)
	s.body = "[" + berlin + "]"
	places, err := c.Lookup(ctx, &httpclient.LookupRequest{Refs: []string{"R62422", " n107775 "}, Language: "en"})
	require.NoError(t, err)
	require.Len(t, places, 1)
	assert.Equal(t, "/osm/lookup", s.path)
	assert.Equal(t, "R62422,N107775", s.query.Get("osm_ids"))

	count := s.count
	for name, refs := range map[string][]string{"none": nil, "type": {"X1"}, "empty": {""}, "no id": {"R"}, "zero": {"R0"}, "sign": {"R+1"}, "list": {"R1,R2"}, "many": make([]string, 51)} {
		_, err := c.Lookup(ctx, &httpclient.LookupRequest{Refs: refs})
		assert.ErrorIs(t, err, llm.ErrBadParameter, name)
	}
	assert.Equal(t, count, s.count)
}

func Test_Policy(t *testing.T) {
	ctx := t.Context()
	reverse := func(lat float64) *httpclient.ReverseRequest { return &httpclient.ReverseRequest{Lat: lat, Lon: 13} }

	// It says what the application is
	s, c := newServer(t, client.OptUserAgent("my-app/1.0"))
	s.body = berlin
	_, err := c.Reverse(ctx, reverse(1))
	require.NoError(t, err)
	assert.Equal(t, "my-app/1.0", s.header.Get("User-Agent"))

	// What was asked for isn't again, including when there's none, but is
	// when it failed
	for _, body := range []string{berlin, `{"error":"Unable to geocode"}`} {
		s, c = newServer(t)
		s.body = body
		for range 3 {
			_, err := c.Reverse(ctx, reverse(1))
			assert.Equal(t, body != berlin, err != nil)
		}
		assert.Equal(t, 1, s.count)
		_, _ = c.Reverse(ctx, reverse(2))
		assert.Equal(t, 2, s.count)
	}
	s, c = newServer(t)
	s.status = http.StatusServiceUnavailable
	_, err = c.Reverse(ctx, reverse(1))
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	s.status, s.body = http.StatusOK, berlin
	_, err = c.Reverse(ctx, reverse(1))
	assert.NoError(t, err)

	// At most one request a second, unless it's set
	s.count = 0
	limited, err := httpclient.New(s.URL)
	require.NoError(t, err)
	start := time.Now()
	for lat := range 2 {
		_, err := limited.Reverse(ctx, reverse(float64(lat)))
		require.NoError(t, err)
	}
	assert.Equal(t, 2, s.count)
	assert.GreaterOrEqual(t, time.Since(start), 950*time.Millisecond)

	// Those which wait can be cancelled
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = limited.Reverse(cancelled, reverse(5))
	assert.ErrorIs(t, err, context.Canceled)
}

func Test_Private(t *testing.T) {
	// Where someone is isn't in the log, the traces, nor the errors
	var out bytes.Buffer
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	opts := []client.ClientOpt{
		client.OptTracer(provider.Tracer("test")),
		client.OptTransport(func(parent http.RoundTripper) http.RoundTripper {
			return transport.NewLogging(&out, parent, true)
		}),
	}
	s, c := newServer(t, opts...)
	s.body = `{"error":"Unable to geocode"}`
	req := &httpclient.ReverseRequest{Lat: 52.987654, Lon: 13.123456}
	var errs []error
	call := func() {
		_, err := c.Reverse(t.Context(), req)
		require.Error(t, err)
		errs = append(errs, err)
		req.Zoom++
	}
	call()
	assert.Equal(t, "52.987654", s.query.Get("lat"), "it's sent")
	s.status, s.location = http.StatusMovedPermanently, "https://other.example.com/reverse?lat=52.987654"
	count := s.count
	call()
	assert.Equal(t, count+1, s.count, "a redirect isn't followed")
	assert.ErrorContains(t, errs[1], "https://other.example.com/reverse")
	s.status, s.location = http.StatusForbidden, ""
	call()
	s.Close()
	call()

	text := out.String()
	for _, err := range errs {
		text += err.Error()
	}
	require.NotEmpty(t, spans.Ended())
	for _, span := range spans.Ended() {
		text += span.Name() + span.Status().Description
		for _, attr := range span.Attributes() {
			text += attr.Value.Emit()
		}
		for _, event := range span.Events() {
			for _, attr := range event.Attributes {
				text += attr.Value.Emit()
			}
		}
	}
	assert.Contains(t, text, "/osm/reverse")
	assert.NotContains(t, text, "52.987654")
	assert.NotContains(t, text, "13.123456")
}

func Test_Errors(t *testing.T) {
	ctx := t.Context()
	s, c := newServer(t)
	lat := 0.0
	for _, test := range []struct {
		status int
		body   string
		err    error
		text   string
	}{
		{400, `{"error":{"code":400,"message":"Bad zoom"}}`, llm.ErrBadParameter, "Bad zoom"},
		{403, `<html>blocked</html>`, llm.ErrForbidden, "usage policy"},
		{404, ``, llm.ErrNotFound, "Not Found"},
		{429, `{}`, llm.ErrServiceUnavailable, "Too Many Requests"},
		{500, `{"error":{"code":500,"message":"Timed out"}}`, llm.ErrServiceUnavailable, "Timed out"},
		{200, `[]`, llm.ErrServiceUnavailable, "isn't Nominatim's"},
	} {
		lat++
		s.status, s.body = test.status, test.body
		_, err := c.Reverse(ctx, &httpclient.ReverseRequest{Lat: lat})
		assert.ErrorIs(t, err, test.err, test.status)
		assert.ErrorContains(t, err, test.text, test.status)
	}
	s.status, s.contentType, s.body = 200, "text/html", "<html></html>"
	_, err := c.Reverse(ctx, &httpclient.ReverseRequest{Lat: 50})
	assert.Error(t, err)
}

func Test_Place(t *testing.T) {
	assert.Equal(t, "N1", httpclient.Place{OSMType: "node", OSMID: 1}.Ref())
	assert.Empty(t, httpclient.Place{}.Ref(), "such as a postal code")
	assert.Empty(t, httpclient.Place{OSMType: "other", OSMID: 1}.Ref())
	assert.Equal(t, "Wylde Green", httpclient.Address{"village": "Wylde Green", "county": "West Midlands"}.City())
	assert.Equal(t, "Sutton", httpclient.Address{"village": "Wylde Green", "town": "Sutton"}.City())
	assert.Empty(t, httpclient.Address(nil).City())
	var f httpclient.Float
	for _, text := range []string{`null`, `""`} {
		require.NoError(t, f.UnmarshalJSON([]byte(text)))
		assert.Zero(t, f)
	}
	assert.Error(t, f.UnmarshalJSON([]byte(`"x"`)))
}
