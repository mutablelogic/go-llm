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
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

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
		Model    string  `json:"model_id,omitempty"`
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

// ListVoices returns the voices of the account, which the model, which must
// be one of ElevenLabs' models which speak, speaks in, which are ElevenLabs'
// own, and those the account has cloned, designed, or added from the voice
// library, each by its voice ID, which it's chosen by, with its name, as its
// title, and what ElevenLabs says of it, such as its accent, gender and age,
// the languages it's verified to speak with the model, and whether it's
// recommended for it, which it is when it's of the best quality with it. With a limit, they're a page at a time, of at most 100, with
// ElevenLabs' token of the next page, and otherwise all of them. Returns
// ErrNotImplemented for a model which doesn't speak, and ErrBadParameter for
// an option which isn't supported.
func (c *Client) ListVoices(ctx context.Context, model llm.Model, opts ...llm.Opt) ([]llm.Voice, string, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, "", llm.ErrBadParameter.With("not a model of this provider")
	} else if m.speech == nil {
		return nil, "", llm.ErrNotImplemented.Withf("%s doesn't speak", m.Name())
	}
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, "", err
	}
	var token string
	var limit uint
	for _, key := range o.Keys() {
		switch key {
		case llm.PageTokenKey:
			token, err = llm.GetOpt[string](o, key)
		case llm.LimitKey:
			limit, err = llm.GetOpt[uint](o, key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported for a list", key)
		}
		if err != nil {
			return nil, "", err
		}
	}
	if !o.Has(llm.LimitKey) {
		voices, err := c.voices(ctx, token, m.Name())
		return voices, "", err
	} else if limit == 0 {
		return nil, token, nil
	}
	page, err := c.voicesPage(ctx, token, min(limit, voicesPageSize))
	if err != nil {
		return nil, "", err
	}
	voices := make([]llm.Voice, 0, len(page.Voices))
	for _, voice := range page.Voices {
		voices = append(voices, voice.voice(m.Name()))
	}
	return voices, page.next(token), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// accountVoices returns all the account's voices, whatever model speaks
// them, such as to find one by its title
func (c *Client) accountVoices(ctx context.Context) ([]llm.Voice, error) {
	return c.voices(ctx, "", "")
}

// voices returns the account's voices, from the page the token says, or the
// first, to the last, with the languages they're verified to speak with the
// model, or any model, when it's empty
func (c *Client) voices(ctx context.Context, token, model string) ([]llm.Voice, error) {
	var result []llm.Voice
	for range voicesPages {
		page, err := c.voicesPage(ctx, token, voicesPageSize)
		if err != nil {
			return nil, err
		}
		for _, voice := range page.Voices {
			result = append(result, voice.voice(model))
		}
		if token = page.next(token); token == "" {
			break
		}
	}
	return result, nil
}

// voicesPage returns the page of the account's voices the token says, or the
// first, of at most the size
func (c *Client) voicesPage(ctx context.Context, token string, size uint) (*voicesResponse, error) {
	query := url.Values{"page_size": {strconv.FormatUint(uint64(size), 10)}, "include_total_count": {"false"}}
	if token != "" {
		query.Set("next_page_token", token)
	}
	var page voicesResponse
	if err := c.DoWithContext(ctx, nil, &page, client.OptAbsPath(c.v2Path("voices")...), client.OptQuery(query)); err != nil {
		return nil, err
	}
	return &page, nil
}

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

// next returns the token of the page after the one, which had the token, or
// empty, after the last, or when ElevenLabs gives the same token again
func (r *voicesResponse) next(token string) string {
	if !r.HasMore || r.NextToken == nil || *r.NextToken == token {
		return ""
	}
	return *r.NextToken
}

// voice returns the voice, by its ID, with its name, as its title, its
// labels, and the languages it's verified to speak with the model, or any
// model, when it's empty, by their locales, such as en-US, or their
// languages, once each
func (v apiVoice) voice(model string) llm.Voice {
	label := func(key string) string {
		return strings.TrimSpace(v.Labels[key])
	}
	voice := llm.Voice{
		ID:       v.ID,
		Title:    strings.TrimSpace(v.Name),
		Category: v.Category,
		Gender:   label("gender"),
		Age:      label("age"),
		Accent:   label("accent"),
		Language: label("language"),
		UseCase:  label("use_case"),
	}
	// It's recommended for the model when it's of the best quality with it,
	// and isn't when it's of the best quality with others, and otherwise
	// isn't known, as ElevenLabs doesn't say
	if model != "" && len(v.Models) > 0 {
		voice.Recommended = types.Ptr(slices.Contains(v.Models, model))
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
		if model != "" && verified.Model != "" && verified.Model != model {
			continue
		}
		language := strings.TrimSpace(verified.Language)
		if verified.Locale != nil && strings.TrimSpace(*verified.Locale) != "" {
			language = strings.TrimSpace(*verified.Locale)
		}
		if language != "" && !slices.Contains(voice.Languages, language) {
			voice.Languages = append(voice.Languages, language)
		}
	}
	return voice
}
