package heartbeat_test

import (
	"testing"
	"time"

	// Packages
	heartbeat "github.com/mutablelogic/go-llm/pkg/heartbeat"
	assert "github.com/stretchr/testify/assert"
)

// testFrom is a fixed reference instant: Friday 6 March 2026, 10:00 UTC.
var testFrom = time.Date(2026, time.March, 6, 10, 0, 0, 0, time.UTC)

///////////////////////////////////////////////////////////////////////////////
// NewTimeSpec[time.Time]

func Test_timespec_001(t *testing.T) {
	// future time.Time: succeeds, all fields pinned, no Weekday or Timezone
	assert := assert.New(t)
	future := time.Date(2030, time.June, 15, 14, 30, 0, 0, time.UTC)
	ts, err := heartbeat.NewTimeSpec(future, nil)
	assert.NoError(err)
	assert.NotNil(ts.Year)
	assert.Equal(2030, *ts.Year)
	assert.Equal([]int{6}, ts.Month)
	assert.Equal([]int{15}, ts.Day)
	assert.Equal([]int{14}, ts.Hour)
	assert.Equal([]int{30}, ts.Minute)
	assert.Empty(ts.Weekday)
	assert.Nil(ts.Loc)
}

func Test_timespec_002(t *testing.T) {
	// past time.Time: error
	assert := assert.New(t)
	past := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	_, err := heartbeat.NewTimeSpec(past, nil)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// NewTimeSpec[string] - RFC3339

func Test_timespec_003(t *testing.T) {
	// RFC3339 future string: succeeds, fields pinned
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("2030-06-15T14:30:00Z", nil)
	assert.NoError(err)
	assert.NotNil(ts.Year)
	assert.Equal(2030, *ts.Year)
	assert.Equal([]int{6}, ts.Month)
	assert.Equal([]int{15}, ts.Day)
	assert.Equal([]int{14}, ts.Hour)
	assert.Equal([]int{30}, ts.Minute)
}

func Test_timespec_004(t *testing.T) {
	// RFC3339 past string: error
	assert := assert.New(t)
	_, err := heartbeat.NewTimeSpec("2020-01-01T00:00:00Z", nil)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// NewTimeSpec[string] - cron

func Test_timespec_005(t *testing.T) {
	// "0 9 * * 1-5": Mon-Fri at 09:00
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("0 9 * * 1-5", nil)
	assert.NoError(err)
	assert.Equal([]int{0}, ts.Minute)
	assert.Equal([]int{9}, ts.Hour)
	assert.Empty(ts.Day)
	assert.Empty(ts.Month)
	assert.Equal([]int{1, 2, 3, 4, 5}, ts.Weekday)
}

func Test_timespec_006(t *testing.T) {
	// "*/15 * * * *": four minute values, everything else wildcard
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("*/15 * * * *", nil)
	assert.NoError(err)
	assert.Equal([]int{0, 15, 30, 45}, ts.Minute)
	assert.Empty(ts.Hour)
	assert.Empty(ts.Day)
	assert.Empty(ts.Month)
	assert.Empty(ts.Weekday)
}

func Test_timespec_007(t *testing.T) {
	// wrong field count: error
	assert := assert.New(t)
	_, err := heartbeat.NewTimeSpec("* * *", nil)
	assert.Error(err)
}

func Test_timespec_008(t *testing.T) {
	// step of zero: error
	assert := assert.New(t)
	_, err := heartbeat.NewTimeSpec("*/0 * * * *", nil)
	assert.Error(err)
}

func Test_timespec_009(t *testing.T) {
	// value out of range (minute 60): error
	assert := assert.New(t)
	_, err := heartbeat.NewTimeSpec("60 * * * *", nil)
	assert.Error(err)
}

func Test_timespec_010(t *testing.T) {
	// impossible schedule (February 31 never exists): no future events, error
	assert := assert.New(t)
	_, err := heartbeat.NewTimeSpec("0 0 31 2 *", nil)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// TimeSpec.Next

func Test_timespec_011(t *testing.T) {
	// pinned one-shot: Next fires at exactly that instant
	assert := assert.New(t)
	future := time.Date(2030, time.June, 15, 14, 30, 0, 0, time.UTC)
	ts, err := heartbeat.NewTimeSpec(future, nil)
	assert.NoError(err)
	got := ts.Next(future.Add(-time.Minute))
	assert.Equal(future, got)
}

func Test_timespec_012(t *testing.T) {
	// "0 9 * * 1-5" from Friday 10:00: next Monday 09:00
	// (9am already passed on Friday; Saturday is not a weekday)
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("0 9 * * 1-5", nil)
	assert.NoError(err)
	got := ts.Next(testFrom)
	want := time.Date(2026, time.March, 9, 9, 0, 0, 0, time.UTC)
	assert.Equal(want, got)
}

func Test_timespec_013(t *testing.T) {
	// "*/15 * * * *" from :00: matches immediately
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("*/15 * * * *", nil)
	assert.NoError(err)
	got := ts.Next(testFrom)
	assert.Equal(testFrom, got)
}

func Test_timespec_014(t *testing.T) {
	// "*/15 * * * *" from :07: next :15
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("*/15 * * * *", nil)
	assert.NoError(err)
	from := time.Date(2026, time.March, 6, 10, 7, 0, 0, time.UTC)
	got := ts.Next(from)
	want := time.Date(2026, time.March, 6, 10, 15, 0, 0, time.UTC)
	assert.Equal(want, got)
}

func Test_timespec_015(t *testing.T) {
	// month rollover: "0 0 1 6 *" from March: first June at midnight
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("0 0 1 6 *", nil)
	assert.NoError(err)
	got := ts.Next(testFrom)
	want := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(want, got)
}

func Test_timespec_016(t *testing.T) {
	// timezone: "0 12 * * *" with America/New_York
	// testFrom is 10:00 UTC = 05:00 EST; next noon NY = 12:00 NY = 17:00 UTC
	assert := assert.New(t)
	nyLoc, err := time.LoadLocation("America/New_York")
	assert.NoError(err)
	ts, err := heartbeat.NewTimeSpec("0 12 * * *", nyLoc)
	assert.NoError(err)
	got := ts.Next(testFrom)
	want := time.Date(2026, time.March, 6, 12, 0, 0, 0, nyLoc)
	assert.Equal(want, got)
}

func Test_timespec_017(t *testing.T) {
	// expired pinned year: zero time
	assert := assert.New(t)
	yr := 2020
	ts := heartbeat.TimeSpec{Year: &yr, Month: []int{1}, Day: []int{1}, Hour: []int{0}, Minute: []int{0}}
	got := ts.Next(testFrom)
	assert.True(got.IsZero())
}

///////////////////////////////////////////////////////////////////////////////
// TimeSpec.Next — day-of-month rollover edge cases

func Test_timespec_021(t *testing.T) {
	// "0 0 31 * *": day 31 skips April (30 days) and lands on May 31
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("0 0 31 * *", nil)
	assert.NoError(err)
	// Start on 1 April 2026; April has no day 31.
	from := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	got := ts.Next(from)
	want := time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC)
	assert.Equal(want, got)
}

func Test_timespec_022(t *testing.T) {
	// "0 0 30,31 * *": skips Feb entirely, lands on March 30
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("0 0 30,31 * *", nil)
	assert.NoError(err)
	// Start on 1 February 2027 (non-leap year).
	from := time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC)
	got := ts.Next(from)
	want := time.Date(2027, time.March, 30, 0, 0, 0, 0, time.UTC)
	assert.Equal(want, got)
}

///////////////////////////////////////////////////////////////////////////////
// TimeSpec.String

func Test_timespec_023(t *testing.T) {
	// cron schedule → String() returns the original expression
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("0 9 * * *", nil)
	assert.NoError(err)
	s := ts.String()
	assert.Equal("0 9 * * *", s)
}

func Test_timespec_025(t *testing.T) {
	// pinned one-shot → String() returns the 6-field cron expression
	assert := assert.New(t)
	future := time.Date(2030, time.June, 15, 14, 30, 0, 0, time.UTC)
	ts, err := heartbeat.NewTimeSpec(future, nil)
	assert.NoError(err)
	s := ts.String()
	assert.Equal("30 14 15 6 * 2030", s)
}

///////////////////////////////////////////////////////////////////////////////
// TimeSpec.Next — sub-minute from must advance past the fractional second

func Test_timespec_027(t *testing.T) {
	// from = 10:07:30 (has sub-minute fraction); "* * * * *" must fire at 10:08, not 10:07
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("* * * * *", nil)
	assert.NoError(err)
	from := time.Date(2026, time.March, 6, 10, 7, 30, 0, time.UTC)
	got := ts.Next(from)
	want := time.Date(2026, time.March, 6, 10, 8, 0, 0, time.UTC)
	assert.Equal(want, got)
}

func Test_timespec_028(t *testing.T) {
	// from = 10:07:00 exactly (no fraction); "* * * * *" may fire at 10:07
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("* * * * *", nil)
	assert.NoError(err)
	from := time.Date(2026, time.March, 6, 10, 7, 0, 0, time.UTC)
	got := ts.Next(from)
	want := time.Date(2026, time.March, 6, 10, 7, 0, 0, time.UTC)
	assert.Equal(want, got)
}

///////////////////////////////////////////////////////////////////////////////
// MarshalJSON / UnmarshalJSON round-trip

func Test_timespec_029(t *testing.T) {
	// cron round-trip: marshal produces {"schedule":"..."}, unmarshal restores fields
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("*/15 * * * *", nil)
	assert.NoError(err)
	data, err := ts.MarshalJSON()
	assert.NoError(err)
	assert.Equal(`{"schedule":"0,15,30,45 * * * *"}`, string(data))

	var ts2 heartbeat.TimeSpec
	assert.NoError(ts2.UnmarshalJSON(data))
	assert.Equal("0,15,30,45 * * * *", ts2.String())
}

func Test_timespec_030(t *testing.T) {
	// RFC3339 round-trip: marshals as {"schedule":"6-field cron"}, unmarshal restores Year
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("2030-06-15T14:30:00Z", nil)
	assert.NoError(err)
	data, err := ts.MarshalJSON()
	assert.NoError(err)
	assert.Equal(`{"schedule":"30 14 15 6 * 2030"}`, string(data))

	var ts2 heartbeat.TimeSpec
	assert.NoError(ts2.UnmarshalJSON(data))
	assert.NotNil(ts2.Year)
	assert.Equal(2030, *ts2.Year)
}

///////////////////////////////////////////////////////////////////////////////
// TimeSpec.String — cron synthesis

func Test_timespec_031(t *testing.T) {
	// "* * * * *": all wildcards round-trip exactly
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("* * * * *", nil)
	assert.NoError(err)
	assert.Equal("* * * * *", ts.String())
}

func Test_timespec_032(t *testing.T) {
	// "0 9 * * 1-5": single values and a contiguous range round-trip exactly
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("0 9 * * 1-5", nil)
	assert.NoError(err)
	assert.Equal("0 9 * * 1-5", ts.String())
}

func Test_timespec_033(t *testing.T) {
	// "*/15 * * * *": expands to [0,15,30,45] — rendered as comma list
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("*/15 * * * *", nil)
	assert.NoError(err)
	assert.Equal("0,15,30,45 * * * *", ts.String())
}

func Test_timespec_034(t *testing.T) {
	// pinned year (from time.Time): CronString synthesizes the 6-field form
	assert := assert.New(t)
	future := time.Date(2030, time.June, 15, 14, 30, 0, 0, time.UTC)
	ts, err := heartbeat.NewTimeSpec(future, nil)
	assert.NoError(err)
	assert.Equal("30 14 15 6 * 2030", ts.String())
}

func Test_timespec_035(t *testing.T) {
	// 6-field cron with pinned year: parsed and round-tripped correctly
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("30 14 15 6 * 2030", nil)
	assert.NoError(err)
	assert.NotNil(ts.Year)
	assert.Equal(2030, *ts.Year)
	assert.Equal([]int{6}, ts.Month)
	assert.Equal([]int{15}, ts.Day)
	assert.Equal([]int{14}, ts.Hour)
	assert.Equal([]int{30}, ts.Minute)
	assert.Equal("30 14 15 6 * 2030", ts.String())
}

func Test_timespec_036(t *testing.T) {
	// 6-field cron with wildcard year ("*"): treated as recurring, no Year pinned
	assert := assert.New(t)
	ts, err := heartbeat.NewTimeSpec("0 9 * * 1-5 *", nil)
	assert.NoError(err)
	assert.Nil(ts.Year)
	// Year was wildcard so String() returns standard 5-field
	assert.Equal("0 9 * * 1-5", ts.String())
}

func Test_timespec_037(t *testing.T) {
	// timezone preserved through a JSON marshal/unmarshal round-trip
	assert := assert.New(t)
	nyLoc, err := time.LoadLocation("America/New_York")
	assert.NoError(err)
	ts, err := heartbeat.NewTimeSpec("0 9 * * 1-5", nyLoc)
	assert.NoError(err)
	data, err := ts.MarshalJSON()
	assert.NoError(err)
	assert.Equal(`{"schedule":"0 9 * * 1-5","timezone":"America/New_York"}`, string(data))

	var ts2 heartbeat.TimeSpec
	assert.NoError(ts2.UnmarshalJSON(data))
	assert.NotNil(ts2.Loc)
	assert.Equal("America/New_York", ts2.Loc.String())
	assert.Equal("0 9 * * 1-5", ts2.String())
}

func Test_timespec_038(t *testing.T) {
	// UnmarshalJSON: legacy bare-string form round-trips correctly
	assert := assert.New(t)
	var ts heartbeat.TimeSpec
	assert.NoError(ts.UnmarshalJSON([]byte(`"0 9 * * 1-5"`)))
	assert.Equal("0 9 * * 1-5", ts.String())
	assert.Nil(ts.Loc)
}

func Test_timespec_039(t *testing.T) {
	// UnmarshalJSON: "Local" timezone rejected
	assert := assert.New(t)
	var ts heartbeat.TimeSpec
	err := ts.UnmarshalJSON([]byte(`{"schedule":"* * * * *","timezone":"Local"}`))
	assert.Error(err)
}

func Test_timespec_040(t *testing.T) {
	// UnmarshalJSON: unknown timezone rejected
	assert := assert.New(t)
	var ts heartbeat.TimeSpec
	err := ts.UnmarshalJSON([]byte(`{"schedule":"* * * * *","timezone":"Not/ATimezone"}`))
	assert.Error(err)
}

func Test_timespec_041(t *testing.T) {
	// UnmarshalJSON: no timezone field → Loc stays nil
	assert := assert.New(t)
	var ts heartbeat.TimeSpec
	assert.NoError(ts.UnmarshalJSON([]byte(`{"schedule":"* * * * *"}`)))
	assert.Nil(ts.Loc)
	assert.Equal("* * * * *", ts.String())
}

func Test_timespec_042(t *testing.T) {
	// NewTimeSpec with "Local" location: error
	assert := assert.New(t)
	_, err := heartbeat.NewTimeSpec("* * * * *", time.Local)
	assert.Error(err)
}

func Test_timespec_043(t *testing.T) {
	// NewTimeSpec time.Time whose Location() is time.Local: error
	assert := assert.New(t)
	localTime := time.Now().In(time.Local)
	_, err := heartbeat.NewTimeSpec(localTime.Add(time.Hour), nil) // loc=nil → uses t.Location() = Local
	assert.Error(err)
}

func Test_timespec_044(t *testing.T) {
	// UnmarshalJSON must NOT validate future occurrence.
	// A one-shot schedule for a time in the past must deserialise without error
	// so that fired heartbeats can be read back from disk.
	assert := assert.New(t)
	past := time.Now().UTC().Add(-time.Hour)
	pastRFC := past.Format(time.RFC3339)
	data := []byte(`{"schedule":"` + pastRFC + `"}`)
	var ts heartbeat.TimeSpec
	assert.NoError(ts.UnmarshalJSON(data))
	// The parsed cron should represent that specific past minute.
	assert.Equal(past.UTC().Minute(), ts.Minute[0])
	assert.Equal(past.UTC().Hour(), ts.Hour[0])
}
