package llm

import "strings"

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
