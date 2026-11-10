package elevenlabs

import (
	"context"
	"net/url"
	"slices"
	"strconv"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

var _ llm.Voicer = (*Client)(nil)

// voicesResponse is a page of the account's voices
type voicesResponse struct {
	Voices    []apiVoice `json:"voices"`
	HasMore   bool       `json:"has_more"`
	NextToken *string    `json:"next_page_token,omitempty"`
}

// apiVoice is a voice, as ElevenLabs lists it
type apiVoice struct {
	ID          string            `json:"voice_id"`
	Name        string            `json:"name"`
	Category    string            `json:"category,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Description *string           `json:"description,omitempty"`
	Preview     *string           `json:"preview_url,omitempty"`
	Models      []string          `json:"high_quality_base_model_ids,omitempty"`
	Verified    []struct {
		Language string  `json:"language"`
		Locale   *string `json:"locale,omitempty"`
	} `json:"verified_languages,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// voicesPageSize is how many voices are listed in a request, which is
	// the most ElevenLabs lists
	voicesPageSize = 100

	// voicesPages is the most pages of voices which are listed, so a list
	// which doesn't end isn't listed forever
	voicesPages = 50
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListVoices returns the account's voices, which are ElevenLabs' own, and
// those the account has cloned, designed, or added from the voice library,
// with what ElevenLabs says of each, such as its accent, gender and age,
// the languages it's verified to speak, and the models it suits best
func (c *Client) ListVoices(ctx context.Context) ([]llm.Voice, error) {
	var result []llm.Voice
	token := ""
	for range voicesPages {
		query := url.Values{"page_size": {strconv.Itoa(voicesPageSize)}, "include_total_count": {"false"}}
		if token != "" {
			query.Set("next_page_token", token)
		}
		var page voicesResponse
		if err := c.DoWithContext(ctx, nil, &page, client.OptAbsPath(c.v2Path("voices")...), client.OptQuery(query)); err != nil {
			return nil, err
		}
		for _, voice := range page.Voices {
			result = append(result, voice.voice())
		}
		if !page.HasMore || page.NextToken == nil || *page.NextToken == "" || *page.NextToken == token {
			return result, nil
		}
		token = *page.NextToken
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// v2Path returns the path of the second version of ElevenLabs' API, which
// lists voices, as the client's endpoint is the first's, such as
// "/v1", with the segments
func (c *Client) v2Path(segments ...string) []any {
	var path []any
	for _, segment := range strings.Split(strings.Trim(c.url.Path, "/"), "/") {
		if segment != "" {
			path = append(path, segment)
		}
	}
	if n := len(path); n > 0 && path[n-1] == "v1" {
		path[n-1] = "v2"
	} else {
		path = append(path, "v2")
	}
	for _, segment := range segments {
		path = append(path, segment)
	}
	return path
}

// voice returns the voice, with its labels, which are in lower case, and
// the languages it's verified to speak, by their locales, such as en-US, or
// their languages, once each
func (v apiVoice) voice() llm.Voice {
	label := func(key string) string {
		return strings.TrimSpace(v.Labels[key])
	}
	voice := llm.Voice{
		Name:     strings.TrimSpace(v.Name),
		ID:       v.ID,
		Category: v.Category,
		Gender:   label("gender"),
		Age:      label("age"),
		Accent:   label("accent"),
		Language: label("language"),
		UseCase:  label("use_case"),
		Models:   v.Models,
	}
	if v.Description != nil {
		voice.Description = strings.TrimSpace(*v.Description)
	}
	if voice.Description == "" {
		voice.Description = label("description")
	}
	if v.Preview != nil {
		voice.Preview = *v.Preview
	}
	for _, verified := range v.Verified {
		language := strings.TrimSpace(verified.Language)
		if verified.Locale != nil && strings.TrimSpace(*verified.Locale) != "" {
			language = strings.TrimSpace(*verified.Locale)
		}
		if language != "" && !slices.Contains(voice.Languages, language) {
			voice.Languages = append(voice.Languages, language)
		}
	}
	if voice.Name == "" {
		voice.Name = v.ID
	}
	return voice
}
