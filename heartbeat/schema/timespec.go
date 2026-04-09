package schema

import (
	"encoding/json"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	// Packages
	llmschema "github.com/mutablelogic/go-llm/kernel/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// TimeSpec describes a recurring or one-shot schedule using cron-like fields.
// Each slice field constrains the schedule to the listed values; an empty
// slice means "any value matches" (wildcard).  Multiple values in a slice are
// treated as OR — the time only needs to match one of them.
// TimeSpec.Next returns the earliest matching moment on or after a given time.
type TimeSpec struct {
	// Year constrains the four-digit calendar year (e.g. 2026). nil = any year.
	Year *int `json:"year,omitempty"`

	// Month constrains the month as numbers 1–12. Empty = any month.
	Month []int `json:"month,omitempty"`

	// Day constrains the day-of-month 1–31. Empty = any day.
	Day []int `json:"day,omitempty"`

	// Weekday constrains the day-of-week: 0 = Sunday … 6 = Saturday. Empty = any.
	Weekday []int `json:"weekday,omitempty"`

	// Hour constrains the hour 0–23. Empty = any hour.
	Hour []int `json:"hour,omitempty"`

	// Minute constrains the minute 0–59. Empty = any minute.
	Minute []int `json:"minute,omitempty"`

	// Loc is the timezone used when evaluating Next. nil means UTC.
	Loc *time.Location `json:"-"`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewTimeSpec creates a TimeSpec from either a time.Time (one-shot, all fields pinned)
// or a cron string (5-field: "minute hour day month weekday").
// loc is the timezone used when evaluating Next; nil means UTC.
//
// Cron field syntax per field:
//
//   - — any value (wildcard)
//     n         — exact value
//     n,m,...   — list of values
//     n-m       — inclusive range
//     */step    — every step-th value across the full range
//     n-m/step  — every step-th value within n–m
//
// Examples:
//
//	NewTimeSpec[time.Time](t, nil)                        → one-shot at the exact minute of t (UTC)
//	NewTimeSpec[string]("0 9 * * 1-5", nil)               → 09:00 every weekday UTC
//	NewTimeSpec[string]("0 9 * * 1-5", londonLoc)         → 09:00 every weekday London time
//	NewTimeSpec[string]("*/15 * * * *", nil)               → every 15 minutes
//	NewTimeSpec[string]("30 14 15 6 * 2030", nil)          → 14:30 on 15 June 2030 (6-field, pinned year)
func NewTimeSpec[T time.Time | string](v T, loc *time.Location) (TimeSpec, error) {
	switch val := any(v).(type) {
	case time.Time:
		if timespec, err := newFromTime(val, loc); err != nil {
			return TimeSpec{}, err
		} else if timespec.IsZero() {
			return TimeSpec{}, llmschema.ErrBadParameter.With("schedule must constrain at least one field")
		} else if timespec.Next(time.Now()).IsZero() {
			return TimeSpec{}, llmschema.ErrBadParameter.Withf("%v has no future events", v)
		} else {
			return timespec, nil
		}
	case string:
		if t, parseErr := time.Parse(time.RFC3339, val); parseErr == nil {
			if timespec, err := newFromTime(t, loc); err != nil {
				return TimeSpec{}, err
			} else if timespec.IsZero() {
				return TimeSpec{}, llmschema.ErrBadParameter.With("schedule must constrain at least one field")
			} else if timespec.Next(time.Now()).IsZero() {
				return TimeSpec{}, llmschema.ErrBadParameter.Withf("%v has no future events", v)
			} else {
				return timespec, nil
			}
		} else {
			if timespec, err := newFromCron(val, loc); err != nil {
				return TimeSpec{}, err
			} else if timespec.IsZero() {
				return TimeSpec{}, llmschema.ErrBadParameter.With("schedule must constrain at least one field")
			} else if timespec.Next(time.Now()).IsZero() {
				return TimeSpec{}, llmschema.ErrBadParameter.Withf("%v has no future events", v)
			} else {
				return timespec, nil
			}
		}
	}
	// Unreachable due to the type constraint, but required for compilation.
	return TimeSpec{}, nil
}

// newFromTime pins every TimeSpec field to the exact values of t.
// The Weekday field is intentionally left empty (it is derivable from year+month+day).
// loc sets the Loc field; loc will use UTC if unset
func newFromTime(t time.Time, loc *time.Location) (TimeSpec, error) {
	if loc != nil && loc.String() == "Local" {
		return TimeSpec{}, llmschema.ErrBadParameter.With("timezone must be a specific IANA name (e.g. Europe/London), not \"Local\"")
	} else if loc == nil {
		loc = time.UTC
	}
	return TimeSpec{
		Year:   types.Ptr(t.Year()),
		Month:  []int{int(t.Month())},
		Day:    []int{t.Day()},
		Hour:   []int{t.Hour()},
		Minute: []int{t.Minute()},
		Loc:    loc,
	}, nil
}

// newFromCron parses a 5-field or 6-field cron expression.
// 5-field order: minute hour day-of-month month day-of-week
// 6-field order: minute hour day-of-month month day-of-week year
// loc will use UTC if unset
func newFromCron(s string, loc *time.Location) (TimeSpec, error) {
	if loc != nil && loc.String() == "Local" {
		return TimeSpec{}, llmschema.ErrBadParameter.With("timezone must be a specific IANA name (e.g. Europe/London), not \"Local\"")
	} else if loc == nil {
		loc = time.UTC
	}

	// Parse fields
	fields := strings.Fields(s)
	if len(fields) != 5 && len(fields) != 6 {
		return TimeSpec{}, llmschema.ErrBadParameter.Withf("cron expression must have 5 or 6 space-separated fields, got %d in %q", len(fields), s)
	}

	// Deal with minute, hour, day, month, weekday fields (in that order)
	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return TimeSpec{}, llmschema.ErrBadParameter.Withf("cron minute %q: %v", fields[0], err)
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return TimeSpec{}, llmschema.ErrBadParameter.Withf("cron hour %q: %v", fields[1], err)
	}
	day, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return TimeSpec{}, llmschema.ErrBadParameter.Withf("cron day %q: %v", fields[2], err)
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return TimeSpec{}, llmschema.ErrBadParameter.Withf("cron month %q: %v", fields[3], err)
	}
	weekday, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return TimeSpec{}, llmschema.ErrBadParameter.Withf("cron weekday %q: %v", fields[4], err)
	}

	// Deal with year
	var year *int
	if len(fields) == 6 && fields[5] != "*" {
		y, err := strconv.Atoi(fields[5])
		if err != nil || y < 1970 {
			return TimeSpec{}, llmschema.ErrBadParameter.Withf("cron year %q: must be * or a year >= 1970", fields[5])
		}
		year = &y
	}

	// Return success
	return TimeSpec{
		Year:    year,
		Month:   month,
		Day:     day,
		Weekday: weekday,
		Hour:    hour,
		Minute:  minute,
		Loc:     loc,
	}, nil
}

// parseCronField expands a single cron field into a sorted, deduplicated slice
// of integers within [min, max].  Returns nil for pure wildcards ("*").
func parseCronField(s string, min, max int) ([]int, error) {
	if s == "*" {
		return nil, nil
	}
	var result []int
	for _, part := range strings.Split(s, ",") {
		vals, err := parseCronPart(part, min, max)
		if err != nil {
			return nil, err
		}
		result = append(result, vals...)
	}
	sort.Ints(result)
	return dedupInts(result), nil
}

// parseCronPart parses a single range/step expression (no commas).
func parseCronPart(s string, min, max int) ([]int, error) {
	step := 1
	if i := strings.LastIndex(s, "/"); i >= 0 {
		var err error
		step, err = strconv.Atoi(s[i+1:])
		if err != nil || step <= 0 {
			return nil, llmschema.ErrBadParameter.Withf("invalid step %q", s[i+1:])
		}
		s = s[:i]
	}

	start, end := min, max
	if s != "*" {
		if i := strings.Index(s, "-"); i >= 0 {
			var err error
			start, err = strconv.Atoi(s[:i])
			if err != nil {
				return nil, llmschema.ErrBadParameter.Withf("invalid range start %q", s[:i])
			}
			end, err = strconv.Atoi(s[i+1:])
			if err != nil {
				return nil, llmschema.ErrBadParameter.Withf("invalid range end %q", s[i+1:])
			}
		} else {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, llmschema.ErrBadParameter.Withf("invalid value %q", s)
			}
			start, end = v, v
		}
	}

	if start < min || end > max || start > end {
		return nil, llmschema.ErrBadParameter.Withf("value %d-%d out of range [%d-%d]", start, end, min, max)
	}

	var result []int
	for v := start; v <= end; v += step {
		result = append(result, v)
	}
	return result, nil
}

// dedupInts removes consecutive duplicates from a sorted slice.
func dedupInts(vals []int) []int {
	if len(vals) == 0 {
		return vals
	}
	out := vals[:1]
	for _, v := range vals[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// IsZero returns true if ts has no constraints
func (ts TimeSpec) IsZero() bool {
	return ts.Year == nil && len(ts.Month) == 0 && len(ts.Day) == 0 && len(ts.Weekday) == 0 && len(ts.Hour) == 0 && len(ts.Minute) == 0
}

// Next returns the earliest time on or after from that satisfies every
// field of ts.  The returned time is expressed in ts.Loc (UTC if nil).
// Returns the zero time.Time when no match exists within a four-year window
// (e.g. an impossible Day+Weekday combination).
func (ts TimeSpec) Next(from time.Time) time.Time {
	loc := time.UTC
	if ts.Loc != nil {
		loc = ts.Loc
	}

	// Work in the target timezone, precision = 1 minute.
	// Advance past any sub-minute fraction so Next always returns a time >= from.
	t := from.In(loc).Truncate(time.Minute)
	if t.Before(from.In(loc)) {
		t = t.Add(time.Minute)
	}
	var limit time.Time
	if ts.Year != nil {
		// Pinned year: search up to the end of that year, however far away it is.
		limit = time.Date(*ts.Year, 12, 31, 23, 59, 0, 0, loc)
	} else {
		limit = t.AddDate(4, 0, 0)
	}

	for !t.After(limit) {
		y, mo, d, h, mi := t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute()
		wd := int(t.Weekday())

		// ── Year ──────────────────────────────────────────────────────────
		if ts.Year != nil {
			if y < *ts.Year {
				t = time.Date(*ts.Year, 1, 1, 0, 0, 0, 0, loc)
				continue
			}
			if y > *ts.Year {
				return time.Time{} // fixed year already passed
			}
		}

		// ── Month ─────────────────────────────────────────────────────────
		if len(ts.Month) > 0 {
			if next, ok := nextIn(ts.Month, mo); !ok {
				if ts.Year != nil {
					return time.Time{} // fixed year has no more matching months
				}
				t = time.Date(y+1, time.Month(next), 1, 0, 0, 0, 0, loc)
				continue
			} else if next != mo {
				t = time.Date(y, time.Month(next), 1, 0, 0, 0, 0, loc)
				continue
			}
		}

		// ── Day-of-month ───────────────────────────────────────────────────
		if len(ts.Day) > 0 && !slices.Contains(ts.Day, d) {
			next, ok := nextIn(ts.Day, d)
			if !ok {
				// No more valid days this month; jump to the first of next month
				// and let the loop re-evaluate — avoids landing on an invalid
				// day (e.g. day 31 in April).
				t = time.Date(y, time.Month(mo+1), 1, 0, 0, 0, 0, loc)
			} else {
				t = time.Date(y, time.Month(mo), next, 0, 0, 0, 0, loc)
			}
			continue
		}

		// ── Weekday ────────────────────────────────────────────────────────
		if len(ts.Weekday) > 0 && !slices.Contains(ts.Weekday, wd) {
			next, ok := nextIn(ts.Weekday, wd)
			var delta int
			if ok {
				delta = next - wd
			} else {
				// Wrap around the week.
				delta = (next - wd + 7) % 7
				if delta == 0 {
					delta = 7
				}
			}
			t = time.Date(y, time.Month(mo), d+delta, 0, 0, 0, 0, loc)
			continue
		}

		// ── Hour ───────────────────────────────────────────────────────────
		if len(ts.Hour) > 0 {
			if next, ok := nextIn(ts.Hour, h); !ok {
				// No more matching hours today; advance to next day.
				t = time.Date(y, time.Month(mo), d+1, next, 0, 0, 0, loc)
				continue
			} else if next != h {
				t = time.Date(y, time.Month(mo), d, next, 0, 0, 0, loc)
				continue
			}
		}

		// ── Minute ────────────────────────────────────────────────────────
		if len(ts.Minute) > 0 {
			if next, ok := nextIn(ts.Minute, mi); !ok {
				// No more matching minutes this hour; advance to next hour.
				t = time.Date(y, time.Month(mo), d, h+1, next, 0, 0, loc)
				continue
			} else if next != mi {
				t = time.Date(y, time.Month(mo), d, h, next, 0, 0, loc)
				continue
			}
		}

		// All constraints satisfied.
		return t
	}

	return time.Time{} // no match within search window
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFYING AND JSON

// String returns the cron expression for this TimeSpec.
// Recurring schedules produce a 5-field expression: "minute hour day-of-month month day-of-week".
// One-shot schedules with a pinned year produce a 6-field expression: "minute hour day-of-month month day-of-week year".
func (ts TimeSpec) String() string {
	fields := []string{
		cronField(ts.Minute),
		cronField(ts.Hour),
		cronField(ts.Day),
		cronField(ts.Month),
		cronField(ts.Weekday),
	}
	if ts.Year != nil {
		fields = append(fields, strconv.Itoa(*ts.Year))
	}
	return strings.Join(fields, " ")
}

// timeSpecJSON is the stable JSON envelope for TimeSpec.
type timeSpecJSON struct {
	Schedule string `json:"schedule"`
	Timezone string `json:"timezone,omitempty"`
}

// MarshalJSON serialises TimeSpec as {"schedule":"...","timezone":"..."}
// (timezone field omitted when UTC/unset), preserving all information through
// a round-trip.
func (ts TimeSpec) MarshalJSON() ([]byte, error) {
	var tz string
	if ts.Loc != nil && ts.Loc != time.UTC {
		tz = ts.Loc.String()
	}
	return json.Marshal(timeSpecJSON{
		Schedule: ts.String(),
		Timezone: tz,
	})
}

// UnmarshalJSON accepts the canonical {"schedule":"...","timezone":"..."}
// envelope.
func (ts *TimeSpec) UnmarshalJSON(data []byte) error {
	// Object form: expect {"schedule":"...","timezone":"..."}.
	var j timeSpecJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if j.Schedule == "" {
		return llmschema.ErrBadParameter.With("schedule field is required")
	}
	var loc *time.Location
	if j.Timezone != "" {
		if j.Timezone == "Local" {
			return llmschema.ErrBadParameter.With("timezone must be a specific IANA name (e.g. Europe/London), not \"Local\"")
		}
		var locErr error
		loc, locErr = time.LoadLocation(j.Timezone)
		if locErr != nil {
			return llmschema.ErrBadParameter.Withf("unknown timezone %q: %v", j.Timezone, locErr)
		}
	}
	return ts.unmarshalSchedule(j.Schedule, loc)
}

// unmarshalSchedule parses a schedule string (RFC3339 or cron) with the given
// location into ts, without validating future occurrence.
func (ts *TimeSpec) unmarshalSchedule(s string, loc *time.Location) error {
	var parsed TimeSpec
	var err error
	if t, parseErr := time.Parse(time.RFC3339, s); parseErr == nil {
		parsed, err = newFromTime(t, loc)
	} else {
		parsed, err = newFromCron(s, loc)
	}
	if err != nil {
		return err
	}
	if parsed.IsZero() {
		return llmschema.ErrBadParameter.With("schedule must constrain at least one field")
	}
	*ts = parsed
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE HELPERS

// nextIn returns the smallest value in vals that is >= cur.
// Returns (value, true) when one is found.
// When all values are < cur it returns (min(vals), false) to signal that the
// parent time unit must advance (wrap-around).
// An empty vals slice always matches: returns (cur, true).
func nextIn(vals []int, cur int) (int, bool) {
	if len(vals) == 0 {
		return cur, true
	}
	best := -1
	for _, v := range vals {
		if v >= cur && (best == -1 || v < best) {
			best = v
		}
	}
	if best != -1 {
		return best, true
	}
	// All values are less than cur; return the minimum to be used after rollover.
	min := vals[0]
	for _, v := range vals[1:] {
		if v < min {
			min = v
		}
	}
	return min, false
}

// cronField encodes a single TimeSpec int slice as a cron field token.
// nil / empty → "*", single value → "n", contiguous range → "n-m",
// otherwise → comma-separated list.
func cronField(vals []int) string {
	if len(vals) == 0 {
		return "*"
	}
	if len(vals) == 1 {
		return strconv.Itoa(vals[0])
	}
	// Contiguous range?
	isRange := true
	for i := 1; i < len(vals); i++ {
		if vals[i] != vals[i-1]+1 {
			isRange = false
			break
		}
	}
	if isRange {
		return strconv.Itoa(vals[0]) + "-" + strconv.Itoa(vals[len(vals)-1])
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}
