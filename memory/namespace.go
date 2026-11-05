package memory

import (
	"fmt"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Name is the name of the namespace of the memory's tools
const Name = "memory"

var _ toolkit.Namespace = (*Memory)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*Memory) Name() string {
	return Name
}

// SystemPrompt returns the time, in the person's time zone, when it's known,
// or the system's, and the details which are known, and which aren't, so
// the model asks for them, and remembers them. The details which are
// declined aren't in it, so they aren't asked for.
func (m *Memory) SystemPrompt() string {
	details := m.Details()
	now := m.now()
	if zone := details.Zone(); zone != nil {
		now = now.In(zone)
	}
	return systemPrompt(details, now)
}

// Notify does nothing, as the memory's tools don't change
func (*Memory) Notify(func(eventschema.EventAction, string)) {}

// Tools returns the memory's tools
func (m *Memory) Tools() []llm.Tool {
	return []llm.Tool{&setDetail{memory: m}}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// systemPrompt returns the time, the details which are known, and those
// which aren't: the assistant's name, which the model asks for first, the
// person's, which it may ask for, and the rest, which it learns from the
// conversation, and how to remember them
func systemPrompt(details schema.Details, now time.Time) string {
	sentences := []string{currentTime(now)}
	if name, ok := details.Value(schema.FieldAssistant); ok {
		sentences = append(sentences, fmt.Sprintf("Your name is %s.", name))
	}

	// The person, by their name, when it's known, and then as they
	person := "The person you're talking to"
	if name, ok := details.Value(schema.FieldName); ok {
		sentences = append(sentences, fmt.Sprintf("You're talking to %s.", name))
		person = "They"
	}
	if location, ok := details.Value(schema.FieldLocation); ok {
		sentences = append(sentences, fmt.Sprintf("%s %s in %s.", person, verb(person, "is", "are"), location))
		person = "They"
	}
	if language, ok := details.Value(schema.FieldLanguage); ok {
		sentences = append(sentences, fmt.Sprintf("%s %s %s.", person, verb(person, "speaks", "speak"), language))
	}

	// What isn't known: the assistant asks for its name, before anything
	// else, may ask for the person's, and learns the rest as the
	// conversation makes them clear
	unknown := false
	if details.Get(schema.FieldAssistant) == nil {
		sentences = append(sentences, "You don't have a name yet. Before anything else, ask the person what they'd like to call you.")
		unknown = true
	}
	if details.Get(schema.FieldName) == nil {
		sentences = append(sentences, "You don't know the person's name. You can ask for it once, when it fits the conversation, but don't press them.")
		unknown = true
	}
	var inferred []string
	for _, field := range []schema.Field{schema.FieldLocation, schema.FieldTimezone, schema.FieldLanguage} {
		if details.Get(field) == nil {
			inferred = append(inferred, unknownPhrases[field])
		}
	}
	if len(inferred) > 0 {
		sentences = append(sentences, fmt.Sprintf("You don't know %s. Don't ask, but when the conversation makes one clear, such as the language they write in, remember it.", join(inferred)))
		unknown = true
	}
	if unknown {
		sentences = append(sentences, "Remember what you learn, or that the person would rather not say, with the set_detail tool, in the memory namespace.")
	}
	return strings.Join(sentences, " ")
}

// unknownPhrases are how what isn't known of each field, which is learnt
// from the conversation, is said
var unknownPhrases = map[schema.Field]string{
	schema.FieldLocation: "where they are",
	schema.FieldTimezone: "their time zone",
	schema.FieldLanguage: "the language they speak",
}

// verb returns the verb for the person, which is singular, unless they're
// they
func verb(person, singular, plural string) string {
	if person == "They" {
		return plural
	}
	return singular
}

// join returns the phrases, separated by commas, with "or" before the last
func join(phrases []string) string {
	if len(phrases) < 2 {
		return strings.Join(phrases, "")
	}
	return strings.Join(phrases[:len(phrases)-1], ", ") + " or " + phrases[len(phrases)-1]
}

// currentTime returns the sentence of the time, with the day of the week, the
// date, the time of day, and the time zone, by its name, such as
// Europe/Berlin, with its abbreviation, when its name is known, or by its
// abbreviation, such as CET, otherwise, and its offset from UTC
func currentTime(now time.Time) string {
	zone, _ := now.Zone()
	if name := now.Location().String(); name != "Local" && name != zone {
		zone = fmt.Sprintf("%s (%s)", name, zone)
	}
	return fmt.Sprintf("The current date and time is %s, in the %s time zone, UTC%s.",
		now.Format("Monday, 2 January 2006, 15:04"), zone, now.Format("-07:00"))
}
