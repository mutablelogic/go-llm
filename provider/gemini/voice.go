package gemini

import (
	"context"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// voices are the prebuilt voices of Gemini's speech models, which are
// fixed, with their styles, in the order Google lists them. Its extended
// voice library, of hundreds more, isn't listed.
// Reference: https://ai.google.dev/gemini-api/docs/speech-generation
var voices = []struct{ name, style string }{
	{"Zephyr", "Bright"}, {"Puck", "Upbeat"}, {"Charon", "Informative"}, {"Kore", "Firm"},
	{"Fenrir", "Excitable"}, {"Leda", "Youthful"}, {"Orus", "Firm"}, {"Aoede", "Breezy"},
	{"Callirrhoe", "Easy-going"}, {"Autonoe", "Bright"}, {"Enceladus", "Breathy"}, {"Iapetus", "Clear"},
	{"Umbriel", "Easy-going"}, {"Algieba", "Smooth"}, {"Despina", "Smooth"}, {"Erinome", "Clear"},
	{"Algenib", "Gravelly"}, {"Rasalgethi", "Informative"}, {"Laomedeia", "Upbeat"}, {"Achernar", "Soft"},
	{"Alnilam", "Firm"}, {"Schedar", "Even"}, {"Gacrux", "Mature"}, {"Pulcherrima", "Forward"},
	{"Achird", "Friendly"}, {"Zubenelgenubi", "Casual"}, {"Vindemiatrix", "Gentle"}, {"Sadachbia", "Lively"},
	{"Sadaltager", "Knowledgeable"}, {"Sulafat", "Warm"},
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListVoices returns the voices the model speaks in, which are Gemini's
// prebuilt voices, which it documents, each with its style. The model
// detects the language it's to speak. Returns ErrNotImplemented for a model
// which doesn't speak.
func (c *Client) ListVoices(_ context.Context, model llm.Model, opts ...llm.Opt) (*llm.VoiceList, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, llm.ErrBadParameter.With("not a model of this provider")
	} else if !m.speaks() {
		return nil, llm.ErrNotImplemented.Withf("%s doesn't speak", m.Name())
	}
	result := make([]llm.Voice, 0, len(voices))
	for _, voice := range voices {
		result = append(result, llm.Voice{ID: voice.name, Title: voice.name, Description: voice.style})
	}
	return llm.PagedVoices(result, opts...)
}
