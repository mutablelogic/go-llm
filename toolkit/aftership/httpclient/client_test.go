package httpclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	httpclient "github.com/mutablelogic/go-llm/toolkit/aftership/httpclient"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// request is a request received by the fake AfterShip server
type request struct {
	method string
	path   string
	query  map[string][]string
	header http.Header
	body   string
}

// server is a fake AfterShip server, which returns the status, and the body,
// and the last request it received
type server struct {
	*httptest.Server
	status int
	body   string
	last   *request
	count  int
}

func newServer(t *testing.T) (*server, *httpclient.Client) {
	t.Helper()
	s := &server{status: http.StatusOK}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		s.last = &request{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.Query(), header: r.Header, body: string(data)}
		s.count++
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(s.status)
		w.Write([]byte(s.body))
	}))
	t.Cleanup(s.Close)
	c, err := httpclient.New(" test-key ", client.OptEndpoint(s.URL+"/tracking/2026-07"), client.OptRateLimit(0))
	require.NoError(t, err)
	return s, c
}

// tracking is a tracking, as AfterShip returns it
const tracking = `{"meta":{"code":200},"data":{"id":"b5fe23","created_at":"2026-11-20T09:00:00+00:00","tracking_number":"1Z999","slug":"ups","active":true,
	"tag":"InTransit","subtag":"InTransit_001","transit_time":3,"signed_by":null,"shipment_pickup_date":"2026-11-21","custom_fields":null,
	"shipment_weight":{"unit":"kg","value":1.5},"shipment_direction":null,
	"latest_estimated_delivery":{"type":"range","source":"Carrier EDD","datetime_min":"2026-11-25","datetime_max":"2026-11-26"},
	"courier_estimated_delivery_date":{"estimated_delivery_date":"2026-11-25"},
	"last_mile":{"tracking_number":"LM1","slug":"dhl","source":"system"},
	"multi_piece_info":{"type":"master","pieces":[{"tracking_id":"p1","trackable":true}]},
	"checkpoints":[{"created_at":"2026-11-21T10:00:00Z","checkpoint_time":"2026-11-21T10:00:00","tag":"InTransit","message":"Departed","city":"Berlin",
		"coordinate":{"latitude":52.5,"longitude":13.4},"events":[{"code":"departed","reason":{"code":"on_schedule"}}],"source":"carrier"}],
	"a_new_field":{"x":1}}}`

func Test_New(t *testing.T) {
	for _, key := range []string{"", " "} {
		_, err := httpclient.New(key)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	}
	c, err := httpclient.New("key")
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func Test_ListTrackings(t *testing.T) {
	s, c := newServer(t)
	s.body = `{"meta":{"code":200},"data":{"pagination":{"total":2,"next_cursor":"c2","has_next_page":true},"trackings":[{"id":"a","tracking_number":"1"}]}}`
	list, err := c.ListTrackings(context.Background(), &httpclient.ListTrackingsRequest{
		Tag: []httpclient.Tag{httpclient.TagInTransit, httpclient.TagException}, Slug: []string{"dhl", " ", "ups"}, Keyword: "a & b",
		UpdatedAtMin: time.Date(2026, 11, 24, 10, 0, 0, 0, time.FixedZone("", 3600)), ReturnToSender: types.Ptr(false), TransitTime: types.Ptr(0), Limit: 10, Cursor: "c1",
	})
	require.NoError(t, err)
	assert.Equal(t, "GET", s.last.method)
	assert.Equal(t, "/tracking/2026-07/trackings", s.last.path)
	assert.Equal(t, map[string][]string{"tag": {"InTransit,Exception"}, "slug": {"dhl,ups"}, "keyword": {"a & b"}, "updated_at_min": {"2026-11-24T09:00:00Z"}, "return_to_sender": {"false"}, "transit_time": {"0"}, "limit": {"10"}, "cursor": {"c1"}}, s.last.query)
	assert.Equal(t, "test-key", s.last.header.Get("As-Api-Key"))
	assert.Empty(t, s.last.body)
	assert.Equal(t, httpclient.Pagination{Total: 2, NextCursor: "c2", HasNextPage: true}, list.Pagination)
	require.Len(t, list.Trackings, 1)
	assert.Equal(t, "1", list.Trackings[0].TrackingNumber)

	// Every tracking, without a request
	_, err = c.ListTrackings(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, s.last.query)
}

func Test_Tracking(t *testing.T) {
	ctx := context.Background()
	s, c := newServer(t)
	s.body = tracking

	// Got, with what's known of it, and what isn't given is empty
	got, err := c.GetTracking(ctx, "b5/fe 23", &httpclient.GetTrackingRequest{Lang: "en", Fields: []string{"tag", "checkpoints"}})
	require.NoError(t, err)
	assert.Equal(t, "GET", s.last.method)
	assert.Equal(t, "/tracking/2026-07/trackings/b5%2Ffe%2023", s.last.path)
	assert.Equal(t, map[string][]string{"lang": {"en"}, "fields": {"tag,checkpoints"}}, s.last.query)
	assert.Equal(t, "b5fe23", got.ID)
	assert.Equal(t, httpclient.TagInTransit, got.Tag)
	assert.True(t, got.Active)
	assert.Equal(t, 3, types.Value(got.TransitTime))
	assert.Equal(t, time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC), got.CreatedAt.UTC())
	assert.Equal(t, "2026-11-21", got.ShipmentPickupDate)
	assert.Empty(t, got.SignedBy)
	assert.Equal(t, 1.5, got.Weight.Value)
	assert.Equal(t, "2026-11-26", got.LatestEstimated.DatetimeMax)
	assert.Equal(t, "2026-11-25", got.CourierEstimated.Date)
	assert.Nil(t, got.OrderPromised)
	assert.Equal(t, "dhl", got.LastMile.Slug)
	assert.True(t, got.MultiPiece.Pieces[0].Trackable)
	require.Len(t, got.Checkpoints, 1)
	assert.Equal(t, "2026-11-21T10:00:00", got.Checkpoints[0].Time)
	assert.Equal(t, 52.5, got.Checkpoints[0].Coordinate.Latitude)
	assert.Equal(t, "on_schedule", got.Checkpoints[0].Events[0].Reason.Code)

	// Created, with what's set
	s.status = http.StatusCreated
	_, err = c.CreateTracking(ctx, &httpclient.CreateTrackingRequest{TrackingNumber: "1Z999", TrackingMeta: httpclient.TrackingMeta{Slug: types.Ptr("ups"), Title: types.Ptr("Shoes")}})
	require.NoError(t, err)
	assert.Equal(t, "POST", s.last.method)
	assert.Equal(t, "/tracking/2026-07/trackings", s.last.path)
	assert.JSONEq(t, `{"tracking_number":"1Z999","slug":"ups","title":"Shoes"}`, s.last.body)
	assert.Equal(t, "application/json", s.last.header.Get("Content-Type"))
	s.status = http.StatusOK

	// Changed, with what's set, including what's removed
	_, err = c.UpdateTracking(ctx, "a", &httpclient.UpdateTrackingRequest{TrackingMeta: httpclient.TrackingMeta{Note: types.Ptr("")}})
	require.NoError(t, err)
	assert.Equal(t, "PUT", s.last.method)
	assert.Equal(t, "/tracking/2026-07/trackings/a", s.last.path)
	assert.JSONEq(t, `{"note":""}`, s.last.body)

	// Deleted, tracked again, and completed
	_, err = c.DeleteTracking(ctx, "a")
	require.NoError(t, err)
	assert.Equal(t, "DELETE /tracking/2026-07/trackings/a", s.last.method+" "+s.last.path)
	_, err = c.RetrackTracking(ctx, "a")
	require.NoError(t, err)
	assert.Equal(t, "POST /tracking/2026-07/trackings/a/retrack", s.last.method+" "+s.last.path)
	assert.Equal(t, "application/json", s.last.header.Get("Content-Type"))
	_, err = c.CompleteTracking(ctx, "a", &httpclient.CompleteTrackingRequest{Reason: httpclient.ReasonLost, Time: "2026-11-24"})
	require.NoError(t, err)
	assert.Equal(t, "POST /tracking/2026-07/trackings/a/mark-as-completed", s.last.method+" "+s.last.path)
	assert.JSONEq(t, `{"reason":"LOST","event_datetime":"2026-11-24"}`, s.last.body)

	// Not without its identifier, whose path would be every tracking's, or
	// what it needs, and nothing is sent
	before := s.count
	for _, id := range []string{"", " "} {
		_, err = c.GetTracking(ctx, id, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = c.UpdateTracking(ctx, id, &httpclient.UpdateTrackingRequest{})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = c.DeleteTracking(ctx, id)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = c.RetrackTracking(ctx, id)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		_, err = c.CompleteTracking(ctx, id, &httpclient.CompleteTrackingRequest{Reason: httpclient.ReasonLost})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	}
	for _, req := range []*httpclient.CreateTrackingRequest{nil, {}, {TrackingNumber: " "}} {
		_, err = c.CreateTracking(ctx, req)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	}
	_, err = c.UpdateTracking(ctx, "a", nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	for _, req := range []*httpclient.CompleteTrackingRequest{nil, {}} {
		_, err = c.CompleteTracking(ctx, "a", req)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	}
	assert.Equal(t, before, s.count)
}

func Test_Couriers(t *testing.T) {
	ctx := context.Background()
	s, c := newServer(t)
	s.body = `{"meta":{"code":200},"data":{"total":1,"couriers":[{"slug":"dhl","name":"DHL Express","required_fields":["destination_postal_code"],"credentials":{"fields":[]}}]}}`
	couriers, err := c.ListCouriers(ctx, &httpclient.ListCouriersRequest{Active: types.Ptr(true), Slug: []string{"dhl"}})
	require.NoError(t, err)
	assert.Equal(t, "GET /tracking/2026-07/couriers", s.last.method+" "+s.last.path)
	assert.Equal(t, map[string][]string{"active": {"true"}, "slug": {"dhl"}}, s.last.query)
	require.Len(t, couriers, 1)
	assert.Equal(t, "DHL Express", couriers[0].Name)
	assert.Equal(t, []string{"destination_postal_code"}, couriers[0].RequiredFields)

	couriers, err = c.DetectCouriers(ctx, &httpclient.DetectCouriersRequest{TrackingNumber: "123", Slug: []string{"dhl", "ups"}})
	require.NoError(t, err)
	assert.Equal(t, "POST /tracking/2026-07/couriers/detect", s.last.method+" "+s.last.path)
	assert.JSONEq(t, `{"tracking_number":"123","slug":["dhl","ups"]}`, s.last.body)
	assert.Len(t, couriers, 1)
	for _, req := range []*httpclient.DetectCouriersRequest{nil, {TrackingNumber: " "}} {
		_, err = c.DetectCouriers(ctx, req)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	}
}

func Test_Errors(t *testing.T) {
	ctx := context.Background()
	s, c := newServer(t)
	for _, test := range []struct {
		status, code int
		expected     error
	}{
		{400, 4003, llm.ErrConflict},
		{400, 4005, llm.ErrBadParameter},
		{401, 401, llm.ErrForbidden},
		{403, 403, llm.ErrForbidden},
		{404, 4004, llm.ErrNotFound},
		{422, 422, llm.ErrBadParameter},
		{429, 429, llm.ErrServiceUnavailable},
		{500, 500, llm.ErrServiceUnavailable},
	} {
		body, err := json.Marshal(map[string]any{"meta": map[string]any{"code": test.code, "type": "X", "message": "What went wrong."}, "data": map[string]any{}})
		require.NoError(t, err)
		s.status, s.body = test.status, string(body)
		_, err = c.GetTracking(ctx, "a", nil)
		assert.ErrorIs(t, err, test.expected, test.code)
		assert.ErrorContains(t, err, "What went wrong.")
		assert.NotContains(t, err.Error(), "test-key")
	}

	// One which doesn't say what went wrong, such as a proxy's, is an error
	for _, body := range []string{"", "{}", "<html>", `{"meta":{"code":502}}`} {
		s.status, s.body = http.StatusBadGateway, body
		_, err := c.GetTracking(ctx, "a", nil)
		assert.ErrorContains(t, err, "502", body)
	}

	// A response which isn't JSON
	s.status, s.body = http.StatusOK, "not JSON"
	_, err := c.GetTracking(ctx, "a", nil)
	assert.Error(t, err)

	// A request which is cancelled
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.GetTracking(cancelled, "a", nil)
	assert.ErrorIs(t, err, context.Canceled)
}
