package gowhisper

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// transcribeOptions are the options of a transcript which aren't in the
// request: the language, when it's set, and whether the transcript has its
// segments
type transcribeOptions struct {
	language string
	segments bool
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// apply sets the options on the request to transcribe, or translate, with
// the model, or returns an error if any option isn't valid or isn't
// supported. The language is the language the audio is in, which a
// translation is from, into English, rather than it being detected. Speakers
// aren't distinguished, as a model which diarizes, such as
// "ggml-small.en-tdrz", only knows when the speaker changes, not who it is.
func (r *transcribeRequest) apply(model string, translate bool, opts ...llm.Opt) (transcribeOptions, error) {
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
			var speakers bool
			if speakers, err = llm.GetOpt[bool](o, key); err == nil && speakers {
				err = llm.ErrBadParameter.Withf("option %q: model %q doesn't distinguish speakers", key, model)
			}
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by gowhisper for transcription", key)
		}
		if err != nil {
			return options, err
		}
	}
	return options, nil
}
