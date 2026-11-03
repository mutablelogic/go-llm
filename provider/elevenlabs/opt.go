package elevenlabs

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// transcribeOptions are the options of a transcript which aren't in the
// request: the language, when it's set, and whether the transcript has its
// segments, and their speakers
type transcribeOptions struct {
	language           string
	segments, speakers bool
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// apply sets the options on the request to transcribe, or returns an error
// if any option isn't valid or isn't supported. Speakers are distinguished
// with diarization, and segments are of the words, which have their times.
func (r *transcribeRequest) apply(opts ...llm.Opt) (transcribeOptions, error) {
	var options transcribeOptions
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return options, err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.LanguageKey:
			if options.language, err = llm.GetOpt[string](o, key); err == nil {
				// The language is checked as WithLanguage checks it, as it
				// can be set with WithOpt, such as from a request
				if _, err = llm.ApplyOpts(llm.WithLanguage(options.language)); err == nil {
					r.Language = options.language
				}
			}
		case llm.TemperatureKey:
			var temperature float64
			if temperature, err = llm.GetOpt[float64](o, key); err == nil {
				r.Temperature = &temperature
			}
		case llm.SegmentsKey:
			options.segments, err = llm.GetOpt[bool](o, key)
		case llm.SpeakersKey:
			if options.speakers, err = llm.GetOpt[bool](o, key); err == nil {
				r.Diarize = options.speakers
			}
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by elevenlabs for transcription", key)
		}
		if err != nil {
			return options, err
		}
	}
	if options.segments || options.speakers {
		r.Timestamps = "word"
	}
	return options, nil
}
