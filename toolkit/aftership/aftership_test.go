package aftership_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	aftership "github.com/mutablelogic/go-llm/toolkit/aftership"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// server is a fake AfterShip server, with the responses to requests, by their
// method and path, and the requests it received
type server struct {
	*httptest.Server
	responses map[string]string
	status    map[string]int
	requests  []string
	queries   []string
	bodies    []string
}

// private is what a tracking has of who it's for
const private = `"customers":[{"name":"PRIVATE","email":"PRIVATE","phone_number":"PRIVATE"}],"subscribed_emails":["PRIVATE"],"subscribed_smses":["PRIVATE"],"signed_by":"PRIVATE",
	"destination_raw_location":"PRIVATE","destination_postal_code":"PRIVATE","origin_raw_location":"PRIVATE","tracking_key":"PRIVATE","tracking_account_number":"PRIVATE",
	"custom_fields":{"x":"PRIVATE"},"proof_of_delivery":[{"type":"signature","url":"PRIVATE"}],"aftership_tracking_url":"PRIVATE"`

// shoes, and books, are trackings, as AfterShip returns them
const shoes = `{"id":"t1","tracking_number":"1Z","title":"Shoes","slug":"ups","tag":"InTransit","subtag_message":"Departed Facility","active":true,"updated_at":"2026-11-24T10:00:00Z","created_at":"2026-11-20T10:00:00Z",
	"latest_estimated_delivery":{"type":"range","datetime_min":"2026-11-25","datetime_max":"2026-11-26"},"courier_estimated_delivery_date":{"estimated_delivery_date":"2026-11-30"},
	"origin_city":"Hamburg","origin_country_region":"DEU","destination_city":"Berlin","destination_country_region":"DEU","last_mile":{"slug":"dhl","tracking_number":"LM"},
	"checkpoints":[{"checkpoint_time":"2026-11-21T10:00:00","tag":"InTransit","message":"Departed","city":"Hamburg","country_region":"DEU"},{"message":"Arrived","location":"PRIVATE 1","city":"Berlin","postal_code":"PRIVATE","country_region":"DEU","country_region_name":"PRIVATE"}],"pickup_location":"PRIVATE 2","pickup_note":"PRIVATE",` + private + `}`
const books = `{"id":"t2","tracking_number":"2B","title":"2B","slug":"dhl","tag":"Delivered","shipment_delivery_date":"2026-11-23","courier_estimated_delivery_date":{"estimated_delivery_date":"2026-11-22"},` + private + `}`

func newServer(t *testing.T) (*server, *toolkit.Toolkit) {
	t.Helper()
	s := &server{responses: map[string]string{}, status: map[string]int{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		key := r.Method + " " + r.URL.Path
		s.requests, s.queries, s.bodies = append(s.requests, key), append(s.queries, r.URL.RawQuery), append(s.bodies, string(data))
		w.Header().Set("Content-Type", "application/json")
		if status, ok := s.status[key]; ok {
			w.WriteHeader(status)
		} else if _, ok := s.responses[key]; !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"meta":{"code":4004,"message":"Tracking does not exist."},"data":{}}`)
			return
		}
		fmt.Fprint(w, s.responses[key])
	}))
	t.Cleanup(s.Close)
	ns, err := aftership.New("key", client.OptEndpoint(s.URL), client.OptRateLimit(0))
	require.NoError(t, err)
	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { tk.Close() })
	require.NoError(t, tk.Add(ns))
	return s, tk
}

// data returns the response with the data
func data(text string) string { return `{"meta":{"code":200},"data":` + text + `}` }

// page returns the response with the trackings, of how many there are, and the
// cursor of the next
func page(total int, cursor string, trackings ...string) string {
	return data(fmt.Sprintf(`{"pagination":{"total":%d,"next_cursor":%q,"has_next_page":%v},"trackings":[%s]}`, total, cursor, cursor != "", strings.Join(trackings, ",")))
}

// last returns the last request
func (s *server) last() (request, query, body string) {
	n := len(s.requests) - 1
	return s.requests[n], s.queries[n], s.bodies[n]
}

func call(t *testing.T, tk *toolkit.Toolkit, name, input string) (map[string]any, string, error) {
	t.Helper()
	result, err := tk.CallTool(t.Context(), "aftership:"+name, json.RawMessage(input))
	if err != nil {
		return nil, "", err
	}
	assert.NotContains(t, string(result), "PRIVATE")
	var value map[string]any
	require.NoError(t, json.Unmarshal(result, &value))
	return value, string(result), nil
}

func Test_Namespace(t *testing.T) {
	_, err := aftership.New(" ")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	ns, err := aftership.New("key")
	require.NoError(t, err)
	assert.Equal(t, "aftership", ns.Name())
	assert.NotEmpty(t, ns.Description())
	assert.Contains(t, ns.SystemPrompt(), "https://admin.aftership.com/tracking/notifications/webhooks")
	assert.Contains(t, ns.SystemPrompt(), "webhook:create")
	var names []string
	for _, tool := range ns.Tools() {
		names = append(names, tool.Name())
		assert.NotEmpty(t, tool.Description())
	}
	assert.Equal(t, []string{"list", "create", "update", "delete"}, names)
	assert.Same(t, ns.Tools()[0], ns.Tools()[0])
}

func Test_List(t *testing.T) {
	s, tk := newServer(t)

	// More than one is in short, with when each is expected, or was delivered
	s.responses["GET /trackings"] = page(5, "next", shoes, books)
	result, text, err := call(t, tk, "list", `{"q":"a b","status":"InTransit","courier":"ups","tracking_number":"1Z","cursor":"c"}`)
	require.NoError(t, err)
	_, query, _ := s.last()
	for _, expected := range []string{"keyword=a+b", "tag=InTransit", "slug=ups", "tracking_numbers=1Z", "cursor=c", "limit=20"} {
		assert.Contains(t, query, expected)
	}
	assert.Len(t, s.requests, 1)
	assert.Equal(t, 5.0, result["count"])
	assert.Equal(t, "next", result["next_cursor"])
	assert.NotContains(t, text, "checkpoints")
	assert.NotContains(t, result, "couriers")
	trackings := result["trackings"].([]any)
	assert.Equal(t, map[string]any{"id": "t1", "tracking_number": "1Z", "title": "Shoes", "courier": "ups", "status": "InTransit", "message": "Departed Facility", "estimated_delivery": "2026-11-25 to 2026-11-26", "updated": "2026-11-24T10:00:00Z"}, trackings[0])
	assert.Equal(t, map[string]any{"id": "t2", "tracking_number": "2B", "courier": "dhl", "status": "Delivered", "delivered": "2026-11-23"}, trackings[1])

	// The only one is in full
	s.responses["GET /trackings"], s.responses["GET /trackings/t1"] = page(1, "", shoes), data(shoes)
	for _, input := range []string{`{}`, `{"id":" t1 "}`, `{"type":"tracking","tracking_number":"1Z"}`} {
		result, _, err = call(t, tk, "list", input)
		require.NoError(t, err, input)
		request, _, _ := s.last()
		assert.Equal(t, "GET /trackings/t1", request)
		assert.Equal(t, 1.0, result["count"])
		one := result["trackings"].([]any)[0].(map[string]any)
		assert.Equal(t, "Hamburg, DEU", one["from"])
		assert.Equal(t, "Berlin, DEU", one["to"])
		assert.Equal(t, map[string]any{"courier": "dhl", "tracking_number": "LM"}, one["last_mile"])
		assert.Equal(t, []any{
			map[string]any{"time": "2026-11-21T10:00:00", "status": "InTransit", "message": "Departed", "location": "Hamburg, DEU"},
			map[string]any{"message": "Arrived", "location": "Berlin, DEU"},
		}, one["checkpoints"])
		assert.NotContains(t, one, "active")
		assert.NotContains(t, one, "tracking_stopped")
	}

	// What happened is kept, when the rest is left out to fit a model's context
	_, text, err = call(t, tk, "list", `{"id":"t1"}`)
	require.NoError(t, err)
	short, err := toolkit.TruncateResult(t.Context(), json.RawMessage(text), uint(len(text)*3/4), func(_ context.Context, text string) (uint, error) { return uint(len(text)), nil })
	require.NoError(t, err)
	assert.Contains(t, string(short), "left out")
	assert.Contains(t, string(short), "Departed")
	assert.Contains(t, string(short), "Arrived")

	// One which is no longer tracked
	s.responses["GET /trackings/t2"] = data(books)
	result, _, err = call(t, tk, "list", `{"id":"t2"}`)
	require.NoError(t, err)
	one := result["trackings"].([]any)[0].(map[string]any)
	assert.Equal(t, true, one["tracking_stopped"])
	assert.Equal(t, []any{}, one["checkpoints"])

	// None is an empty list
	s.responses["GET /trackings"] = page(0, "")
	_, text, err = call(t, tk, "list", `{"limit":100}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"count":0,"trackings":[]}`, text)

	// Those which changed since
	_, _, err = call(t, tk, "list", `{"since":"1d"}`)
	require.NoError(t, err)
	_, query, _ = s.last()
	assert.Contains(t, query, "updated_at_min="+time.Now().Add(-24*time.Hour).UTC().Format("2006-01-02"))

	// Not what isn't valid, which isn't sent
	before := len(s.requests)
	for _, input := range []string{`{"since":"60"}`, `{"since":"yesterday"}`, `{"type":"parcel"}`, `{"status":"Lost"}`, `{"limit":101}`, `{"limit":"x"}`} {
		_, _, err = call(t, tk, "list", input)
		assert.Error(t, err, input)
	}
	assert.Len(t, s.requests, before)
	_, _, err = call(t, tk, "list", `{"id":"unknown"}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_ListCouriers(t *testing.T) {
	const ups, dhl = `{"slug":"ups","name":"UPS","required_fields":["destination_postal_code"]}`, `{"slug":"dhl-germany","name":"Deutsche Post DHL","other_name":"DHL Paket"}`
	s, tk := newServer(t)
	s.responses["GET /couriers"] = data(`{"total":2,"couriers":[` + ups + `,` + dhl + `]}`)

	// Those whose name has the words, in short, those which need an account
	// last, and the shortest code first
	const api, sftp, plain = `{"slug":"dhl-api","name":"DHL API"}`, `{"slug":"dhl-x","name":"DHL X","credentials":{"fields":[{"name":"key","required":true}]}}`, `{"slug":"dhl","name":"DHL Express","credentials":{"fields":[]}}`
	s.responses["GET /couriers"] = data(`{"total":5,"couriers":[` + strings.Join([]string{api, sftp, ups, dhl, plain}, ",") + `]}`)
	_, text, err := call(t, tk, "list", `{"type":"courier","q":"DHL"}`)
	require.NoError(t, err)
	_, query, _ := s.last()
	assert.Empty(t, query)
	assert.JSONEq(t, `{"count":4,"couriers":[{"id":"dhl","name":"DHL Express"},{"id":"dhl-germany","name":"Deutsche Post DHL"},{"id":"dhl-x","name":"DHL X","needs_account":true},{"id":"dhl-api","name":"DHL API","needs_account":true}]}`, text)
	result, _, err := call(t, tk, "list", `{"type":"courier","q":"dhl","limit":2}`)
	require.NoError(t, err)
	assert.Equal(t, 4.0, result["count"])
	assert.Len(t, result["couriers"], 2)

	// The only one is in full, whose code, or names, have each word
	for _, q := range []string{"paket", "dhl germany", "germany-dhl", " Deutsche  POST "} {
		_, text, err = call(t, tk, "list", fmt.Sprintf(`{"type":"courier","q":%q}`, q))
		require.NoError(t, err)
		assert.JSONEq(t, `{"count":1,"couriers":[{"id":"dhl-germany","name":"Deutsche Post DHL","other_name":"DHL Paket"}]}`, text, q)
	}
	_, text, err = call(t, tk, "list", `{"type":"courier","q":"none"}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"count":0,"couriers":[]}`, text)

	// Not every courier, as there are too many
	before := len(s.requests)
	for _, input := range []string{`{"type":"courier"}`, `{"type":"courier","q":" "}`} {
		_, _, err = call(t, tk, "list", input)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	}
	assert.Len(t, s.requests, before)

	// The one with the code, and what it needs
	s.responses["GET /couriers"] = data(`{"total":1,"couriers":[` + ups + `]}`)
	_, text, err = call(t, tk, "list", `{"type":"courier","id":"ups"}`)
	require.NoError(t, err)
	_, query, _ = s.last()
	assert.Equal(t, "slug=ups", query)
	assert.Contains(t, text, "destination_postal_code")
	s.responses["GET /couriers"] = data(`{"total":0,"couriers":[]}`)
	_, _, err = call(t, tk, "list", `{"type":"courier","id":"unknown"}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)

	// Those a number could be of
	s.responses["POST /couriers/detect"] = data(`{"total":3,"couriers":[` + api + `,` + ups + `,` + dhl + `]}`)
	result, text, err = call(t, tk, "list", `{"type":"courier","tracking_number":"1Z"}`)
	require.NoError(t, err)
	_, _, body := s.last()
	assert.JSONEq(t, `{"tracking_number":"1Z"}`, body)
	assert.Equal(t, 3.0, result["count"])
	assert.JSONEq(t, `{"count":3,"couriers":[{"id":"ups","name":"UPS"},{"id":"dhl-germany","name":"Deutsche Post DHL"},{"id":"dhl-api","name":"DHL API","needs_account":true}]}`, text)
}

func Test_Create(t *testing.T) {
	s, tk := newServer(t)
	s.responses["POST /trackings"] = data(shoes)
	result, _, err := call(t, tk, "create", `{"tracking_number":" 1Z ","title":"Shoes","courier":"","destination_postal_code":"10115"}`)
	require.NoError(t, err)
	_, _, body := s.last()
	assert.JSONEq(t, `{"tracking_number":"1Z","title":"Shoes","destination_postal_code":"10115"}`, body)
	assert.Equal(t, "t1", result["id"])
	assert.NotContains(t, result, "existing")

	// One which is tracked already is returned, unless it can't be found
	s.status["POST /trackings"], s.responses["POST /trackings"] = 400, `{"meta":{"code":4003,"message":"Tracking already exists."}}`
	s.responses["GET /trackings"] = page(1, "", shoes)
	result, _, err = call(t, tk, "create", `{"tracking_number":"1Z","courier":"ups"}`)
	require.NoError(t, err)
	_, query, _ := s.last()
	assert.Contains(t, query, "tracking_numbers=1Z")
	assert.Contains(t, query, "slug=ups")
	assert.Equal(t, true, result["existing"])
	assert.Equal(t, "t1", result["id"])
	s.responses["GET /trackings"] = page(0, "")
	_, _, err = call(t, tk, "create", `{"tracking_number":"1Z"}`)
	assert.ErrorIs(t, err, llm.ErrConflict)

	// Another error is returned, without looking for one
	s.responses["POST /trackings"] = `{"meta":{"code":4005,"message":"The value of tracking_number is invalid."}}`
	_, _, err = call(t, tk, "create", `{"tracking_number":"x"}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	request, _, _ := s.last()
	assert.Equal(t, "POST /trackings", request)

	// Not without a number
	before := len(s.requests)
	for _, input := range []string{``, `{}`, `{"tracking_number":" "}`} {
		_, _, err = call(t, tk, "create", input)
		assert.Error(t, err, input)
	}
	assert.Len(t, s.requests, before)
}

func Test_Update(t *testing.T) {
	s, tk := newServer(t)
	s.responses["PUT /trackings/t1"], s.responses["POST /trackings/t1/mark-as-completed"] = data(shoes), data(books)

	// Changed, with what's given
	result, _, err := call(t, tk, "update", `{"id":"t1","title":" New ","note":"","courier":"dhl"}`)
	require.NoError(t, err)
	_, _, body := s.last()
	assert.JSONEq(t, `{"title":"New","slug":"dhl"}`, body)
	assert.Equal(t, "t1", result["id"])
	assert.Len(t, s.requests, 1)

	// Completed, without being changed
	for completed, reason := range map[string]string{"delivered": "DELIVERED", "lost": "LOST", "returned": "RETURNED_TO_SENDER"} {
		result, _, err = call(t, tk, "update", fmt.Sprintf(`{"id":"t1","completed":%q,"completed_at":"2026-11-24"}`, completed))
		require.NoError(t, err)
		request, _, body := s.last()
		assert.Equal(t, "POST /trackings/t1/mark-as-completed", request)
		assert.JSONEq(t, `{"reason":"`+reason+`","event_datetime":"2026-11-24"}`, body)
		assert.Equal(t, "Delivered", result["status"])
	}
	assert.Len(t, s.requests, 4)

	// Both, changed first
	_, _, err = call(t, tk, "update", `{"id":"t1","title":"New","completed":"lost"}`)
	require.NoError(t, err)
	assert.Equal(t, []string{"PUT /trackings/t1", "POST /trackings/t1/mark-as-completed"}, s.requests[4:])

	// Which says it was changed, when it isn't completed
	s.status["POST /trackings/t1/mark-as-completed"], s.responses["POST /trackings/t1/mark-as-completed"] = 400, `{"meta":{"code":400,"message":"It can't be."}}`
	_, _, err = call(t, tk, "update", `{"id":"t1","title":"New","completed":"lost"}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.ErrorContains(t, err, "changed, but not completed")
	_, _, err = call(t, tk, "update", `{"id":"t1","completed":"lost"}`)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.NotContains(t, err.Error(), "changed, but")

	// Not without a change, or one which isn't valid, which isn't sent
	before := len(s.requests)
	for _, input := range []string{`{}`, `{"id":" ","title":"x"}`, `{"id":"t1"}`, `{"id":"t1","title":" "}`, `{"id":"t1","completed":"gone"}`, `{"id":"t1","completed_at":"2026-11-24"}`, `{"id":"t1","title":"x","completed_at":"2026-11-24"}`} {
		_, _, err = call(t, tk, "update", input)
		assert.Error(t, err, input)
	}
	assert.Len(t, s.requests, before)
	_, _, err = call(t, tk, "update", `{"id":"unknown","title":"x"}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_Delete(t *testing.T) {
	s, tk := newServer(t)
	s.responses["DELETE /trackings/t1"] = data(shoes)
	result, _, err := call(t, tk, "delete", `{"id":"t1"}`)
	require.NoError(t, err)
	assert.Equal(t, "t1", result["id"])
	_, _, err = call(t, tk, "delete", `{"id":"unknown"}`)
	assert.ErrorIs(t, err, llm.ErrNotFound)
	before := len(s.requests)
	for _, input := range []string{``, `{}`, `{"id":" "}`} {
		_, _, err = call(t, tk, "delete", input)
		assert.Error(t, err, input)
	}
	assert.Len(t, s.requests, before)
}
