package schema

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// TimeSpec is a recurring schedule, as a cron expression of five fields,
// "minute hour day month weekday", such as "0 9 * * 1-5" for nine in the
// morning on weekdays. Each field is the values it matches, or any value,
// when it's empty. It has no time zone: it's the time on a clock, which
// Next finds in a time zone.
type TimeSpec struct {
	Minute  []int // 0-59
	Hour    []int // 0-23
	Day     []int // 1-31
	Month   []int // 1-12
	Weekday []int // 0-6, from Sunday
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// searchYears is how far ahead Next looks for a time which matches, such as
// the 29th of February
const searchYears = 5

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// ParseTimeSpec returns the schedule of the cron expression, whose fields
// are each "*", for any value, a value, a range, such as "1-5", a step, such
// as "*/15" or "0-30/10", or a list of those, such as "0,30". A weekday of 7
// is Sunday, as 0 is. Returns ErrBadParameter when it isn't valid, or never
// matches, such as the 31st of February.
func ParseTimeSpec(s string) (TimeSpec, error) {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return TimeSpec{}, llm.ErrBadParameter.Withf("schedule %q must have five fields, minute hour day month weekday, such as \"0 9 * * 1-5\"", s)
	}
	var ts TimeSpec
	for i, field := range []struct {
		name     string
		min, max int
		values   *[]int
		every    bool
	}{
		{"minute", 0, 59, &ts.Minute, false},
		{"hour", 0, 23, &ts.Hour, false},
		{"day", 1, 31, &ts.Day, true},
		{"month", 1, 12, &ts.Month, false},
		{"weekday", 0, 7, &ts.Weekday, true},
	} {
		values, err := parseField(fields[i], field.min, field.max, field.every)
		if err != nil {
			return TimeSpec{}, llm.ErrBadParameter.Withf("schedule %q has an invalid %s %q: %v", s, field.name, fields[i], err)
		}
		*field.values = values
	}

	// Sunday is 0, or 7
	if slices.Contains(ts.Weekday, 7) {
		ts.Weekday = normalize(append(slices.DeleteFunc(ts.Weekday, func(v int) bool { return v == 7 }), 0))
	}
	if ts.Next(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.UTC).IsZero() {
		return TimeSpec{}, llm.ErrBadParameter.Withf("schedule %q never happens", s)
	}
	return ts, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Next returns the first time which matches the schedule, on a clock in the
// time zone, at or after the time, to the minute, or the zero time, when
// none does within five years. When the clocks change, a time which is
// skipped, as they go forward, happens once, when it would have, on the
// clock as it was, which is after they change, such as 02:30 as 03:30, and a
// time which is repeated, as they go back, happens once, the first time.
// The repeated hour is a time like any other, so a schedule of every
// fifteen minutes happens in it once.
func (ts TimeSpec) Next(from time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}

	// The time on the clock, to the minute, at or after the time, which is
	// searched for the schedule as a time without a zone, in UTC, which
	// has no changes of the clocks
	local := from.In(loc)
	clock := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), 0, 0, time.UTC)
	if local.Second() != 0 || local.Nanosecond() != 0 {
		clock = clock.Add(time.Minute)
	}
	for {
		clock = ts.next(clock)
		if clock.IsZero() {
			return time.Time{}
		}

		// The time in the zone, which is the first of the times on the clock,
		// when it's repeated, which is at or after the time, or none, as it was
		// passed, when the clocks went back
		if t := instant(clock, loc, from); !t.IsZero() {
			return t
		}
		clock = clock.Add(time.Minute)
	}
}

// String returns the schedule as a cron expression
func (ts TimeSpec) String() string {
	return strings.Join([]string{format(ts.Minute), format(ts.Hour), format(ts.Day), format(ts.Month), format(ts.Weekday)}, " ")
}

// MarshalJSON returns the schedule as a cron expression
func (ts TimeSpec) MarshalJSON() ([]byte, error) {
	return json.Marshal(ts.String())
}

// UnmarshalJSON parses the schedule from a cron expression
func (ts *TimeSpec) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseTimeSpec(s)
	if err != nil {
		return err
	}
	*ts = parsed
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// next returns the first time which matches the schedule, at or after the
// time, which is in UTC, to the minute, within five years, or the zero time
func (ts TimeSpec) next(t time.Time) time.Time {
	limit := t.AddDate(searchYears, 0, 0)
	for !t.After(limit) {
		y, mo, d, h, mi := t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute()

		// The month, or the first day of the next which matches
		if next, ok := nextIn(ts.Month, mo); !ok {
			t = time.Date(y+1, time.Month(next), 1, 0, 0, 0, 0, time.UTC)
			continue
		} else if next != mo {
			t = time.Date(y, time.Month(next), 1, 0, 0, 0, 0, time.UTC)
			continue
		}

		// The day, which matches the day of the month, and of the week, or
		// the next day
		if !ts.matchesDay(t) {
			t = time.Date(y, time.Month(mo), d+1, 0, 0, 0, 0, time.UTC)
			continue
		}

		// The hour, or the first of the next day
		if next, ok := nextIn(ts.Hour, h); !ok {
			t = time.Date(y, time.Month(mo), d+1, 0, 0, 0, 0, time.UTC)
			continue
		} else if next != h {
			t = time.Date(y, time.Month(mo), d, next, 0, 0, 0, time.UTC)
			continue
		}

		// The minute, or the first of the next hour
		if next, ok := nextIn(ts.Minute, mi); !ok {
			t = time.Date(y, time.Month(mo), d, h+1, 0, 0, 0, time.UTC)
			continue
		} else if next != mi {
			t = time.Date(y, time.Month(mo), d, h, next, 0, 0, time.UTC)
			continue
		}
		return t
	}
	return time.Time{}
}

// matchesDay returns whether the day matches: its day of the month, and of
// the week, or either, when both are set, as cron does, so "0 0 1 * 1" is
// the first of the month, and every Monday
func (ts TimeSpec) matchesDay(t time.Time) bool {
	day := len(ts.Day) == 0 || slices.Contains(ts.Day, t.Day())
	weekday := len(ts.Weekday) == 0 || slices.Contains(ts.Weekday, int(t.Weekday()))
	if len(ts.Day) > 0 && len(ts.Weekday) > 0 {
		return day || weekday
	}
	return day && weekday
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// instant returns the time in the zone of the time on the clock, which is in
// UTC, at or after the time from, or the zero time, when it's before it. A
// time which the clocks skip, as they go forward, is when it would have been,
// on the clock as it was before, which is after they change, and a time which
// is repeated, as they go back, is the first time, so it's the zero time,
// when the first time is before the time from, rather than the second.
func instant(clock time.Time, loc *time.Location, from time.Time) time.Time {
	t := time.Date(clock.Year(), clock.Month(), clock.Day(), clock.Hour(), clock.Minute(), 0, 0, loc)
	if !sameClock(t, clock) {
		// The clock was skipped, which time.Date may make a time before the
		// clocks changed, rather than after, so it's the time on the clock,
		// at the offset from UTC before they changed, which is three hours
		// before, as no change is longer
		_, offset := time.Date(clock.Year(), clock.Month(), clock.Day(), clock.Hour()-3, clock.Minute(), 0, 0, loc).Zone()
		t = clock.Add(-time.Duration(offset) * time.Second).In(loc)
	} else {
		// The earliest time with the same time on the clock, as it's
		// repeated, when the clocks go back, by as much as two hours
		for _, d := range []time.Duration{2 * time.Hour, time.Hour, 30 * time.Minute} {
			if earlier := t.Add(-d); sameClock(earlier.In(loc), clock) {
				t = earlier
				break
			}
		}
	}
	if t.Before(from) {
		return time.Time{}
	}
	return t
}

// sameClock returns whether the times are the same on a clock, to the
// minute
func sameClock(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay() && a.Hour() == b.Hour() && a.Minute() == b.Minute()
}

// nextIn returns the first of the values at or after the value, and true,
// or the first of the values, and false, when there's none, so the next
// unit, such as the next day, is the next to match. Any value matches, when
// there are none.
func nextIn(values []int, value int) (int, bool) {
	if len(values) == 0 {
		return value, true
	}
	for _, v := range values {
		if v >= value {
			return v, true
		}
	}
	return values[0], false
}

// parseField returns the values of the field, in order, or none, for any
// value, which every value is, too, unless they're kept, as for the day and
// weekday, where any value, "*", isn't the same as every value, such as
// "1-31", as when both are set, either matches
func parseField(s string, min, max int, every bool) ([]int, error) {
	if s == "*" {
		return nil, nil
	}
	var result []int
	for part := range strings.SplitSeq(s, ",") {
		values, err := parsePart(part, min, max)
		if err != nil {
			return nil, err
		}
		result = append(result, values...)
	}
	result = normalize(result)
	if !every && len(result) == max-min+1 {
		return nil, nil
	}
	return result, nil
}

// parsePart returns the values of a value, range or step
func parsePart(s string, min, max int) ([]int, error) {
	step, stepped := 1, false
	if before, after, ok := strings.Cut(s, "/"); ok {
		var err error
		if step, err = strconv.Atoi(after); err != nil || step <= 0 {
			return nil, llm.ErrBadParameter.Withf("invalid step %q", after)
		}
		s, stepped = before, true
	}
	start, end := min, max
	if s != "*" {
		if before, after, ok := strings.Cut(s, "-"); ok {
			var err1, err2 error
			start, err1 = strconv.Atoi(before)
			end, err2 = strconv.Atoi(after)
			if err1 != nil || err2 != nil {
				return nil, llm.ErrBadParameter.Withf("invalid range %q", s)
			}
		} else if v, err := strconv.Atoi(s); err != nil {
			return nil, llm.ErrBadParameter.Withf("invalid value %q", s)
		} else if stepped {
			// A value with a step, such as 5/15, is from it to the end
			start = v
		} else {
			start, end = v, v
		}
	}
	if start < min || end > max || start > end {
		return nil, llm.ErrBadParameter.Withf("%d-%d is outside %d-%d", start, end, min, max)
	}
	var result []int
	for v := start; v <= end; v += step {
		result = append(result, v)
	}
	return result, nil
}

// normalize returns the values sorted, without repeats
func normalize(values []int) []int {
	slices.Sort(values)
	return slices.Compact(values)
}

// format returns the values as a field: "*", a value, a range, such as
// "1-5", or a list, such as "0,30"
func format(values []int) string {
	switch {
	case len(values) == 0:
		return "*"
	case len(values) == 1:
		return strconv.Itoa(values[0])
	case values[len(values)-1]-values[0] == len(values)-1:
		return strconv.Itoa(values[0]) + "-" + strconv.Itoa(values[len(values)-1])
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}
