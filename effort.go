package llm

import (
	"slices"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Efforts are the reasoning efforts which can be asked of a model, from the
// least to the most. A model which hasn't the effort makes the nearest it
// has.
var Efforts = []string{"low", "medium", "high", "xhigh", "max"}

// efforts are the reasoning efforts which providers have, from the least to
// the most, which include those which turn reasoning off, or nearly
var efforts = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// NearestEffort returns the reasoning effort, when it's one of the levels,
// or otherwise the level which is nearest it, and the greater, when two are
// as near, such as "high" for "medium", of "low" and "high". It's the effort
// when there are no levels, or it isn't a known effort, such as "none",
// "minimal", or one of Efforts.
func NearestEffort(effort string, levels ...string) string {
	want := slices.Index(efforts, effort)
	if len(levels) == 0 || want < 0 || slices.Contains(levels, effort) {
		return effort
	}
	result, distance := effort, len(efforts)
	for _, level := range levels {
		i := slices.Index(efforts, level)
		if i < 0 {
			continue
		}
		d := max(i-want, want-i)
		if d < distance || (d == distance && i > want) {
			result, distance = level, d
		}
	}
	return result
}

// MostEffort returns the reasoning effort, or the most, when it's more,
// such as "high" for "max", of a provider whose models have no more than
// the most, as far as it's known
func MostEffort(effort, most string) string {
	if i, j := slices.Index(efforts, effort), slices.Index(efforts, most); i >= 0 && j >= 0 && i > j {
		return most
	}
	return effort
}
