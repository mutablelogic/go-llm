package llm

import "strings"

///////////////////////////////////////////////////////////////////////////////
// TYPES

// VoiceList is a page of a model's voices, in the order its provider lists
// them, with how many there are, and the token of the next page, which is
// empty, after the last
type VoiceList struct {
	Body  []Voice `json:"body,omitempty" help:"The voices, each chosen by its id, or its title"`
	Count uint    `json:"count" help:"How many voices there are, of every page" example:"23"`
	Next  string  `json:"next,omitempty" help:"The opaque token of the next page, which is passed back as it is, when there are more" example:"cGFnZToyNQ"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// FindVoices returns the voices whose title, or ID, has the search, ignoring
// case, such as "matilda" for "Matilda - Knowledgable, Professional", in
// the order they're in, or all of them, when the search is empty
func FindVoices(voices []Voice, search string) []Voice {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return voices
	}
	result := make([]Voice, 0, len(voices))
	for _, voice := range voices {
		if strings.Contains(strings.ToLower(voice.Title), search) || strings.Contains(strings.ToLower(voice.ID), search) {
			result = append(result, voice)
		}
	}
	return result
}

// PagedVoices returns the page of the voices, which the options say, as
// Paged does, with how many there are
func PagedVoices(voices []Voice, opts ...Opt) (*VoiceList, error) {
	page, next, err := Paged(voices, opts...)
	if err != nil {
		return nil, err
	}
	return &VoiceList{Body: page, Count: uint(len(voices)), Next: next}, nil
}
