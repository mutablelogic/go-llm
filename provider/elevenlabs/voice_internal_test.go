package elevenlabs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ListVoices(t *testing.T) {
	// A server with two pages of voices, from the second version of the API
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path+"?"+r.URL.RawQuery)
		assert.Equal(t, "key", r.Header.Get(apiKeyHeader))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("next_page_token") {
		case "":
			w.Write([]byte(`{"voices":[{"voice_id":"21m00Tcm4TlvDq8ikWAM","name":"Rachel","category":"premade",
				"labels":{"accent":"american","age":"young","gender":"female","use_case":"conversational","description":"calm"},
				"description":"Matter-of-fact, personable woman.","preview_url":"https://example.com/rachel.mp3",
				"high_quality_base_model_ids":["eleven_multilingual_v2","eleven_flash_v2_5"],
				"verified_languages":[{"language":"en","model_id":"eleven_multilingual_v2","locale":"en-US"},{"language":"en","model_id":"eleven_flash_v2_5","locale":"en-US"},{"language":"de","model_id":"eleven_multilingual_v2"}]}],
				"has_more":true,"total_count":2,"next_page_token":"page-2"}`))
		default:
			w.Write([]byte(`{"voices":[{"voice_id":"abc","name":" My clone ","category":"cloned","labels":{"description":"warm","language":"de"}}],"has_more":false,"total_count":2,"next_page_token":null}`))
		}
	}))
	t.Cleanup(server.Close)
	c, err := New(server.URL+"/v1", "key")
	require.NoError(t, err)

	model := c.speechModel(apiModel{ID: "eleven_multilingual_v2", Speaks: true})
	voices, err := c.ListVoices(context.Background(), model)
	require.NoError(t, err)
	assert.Empty(t, voices.Next)
	assert.Equal(t, uint(2), voices.Count)
	assert.Equal(t, []llm.Voice{
		{
			ID: "21m00Tcm4TlvDq8ikWAM", Title: "Rachel", Category: "premade",
			Description: "Matter-of-fact, personable woman.", Gender: "female", Age: "young", Accent: "american", UseCase: "conversational",
			Languages: []string{"en-US", "de"}, Recommended: types.Ptr(true), Preview: "https://example.com/rachel.mp3",
		},
		{ID: "abc", Title: "My clone", Category: "cloned", Description: "warm", Language: "de"},
	}, voices.Body)
	assert.Equal(t, []string{
		"/v2/voices?include_total_count=false&page_size=100",
		"/v2/voices?include_total_count=false&next_page_token=page-2&page_size=100",
	}, asked, "each page, with the token of the one before")

	// The languages are those it's verified to speak with the model, and
	// it's recommended for a model when it's of the best quality with it,
	// isn't when it's of the best quality with others, and isn't known when
	// ElevenLabs doesn't say
	voices, err = c.ListVoices(context.Background(), c.speechModel(apiModel{ID: "eleven_flash_v2_5", Speaks: true}))
	require.NoError(t, err)
	assert.Equal(t, []string{"en-US"}, voices.Body[0].Languages)
	assert.Equal(t, types.Ptr(true), voices.Body[0].Recommended)
	assert.Nil(t, voices.Body[1].Recommended)
	voices, err = c.ListVoices(context.Background(), c.speechModel(apiModel{ID: "eleven_v4_turbo", Speaks: true}))
	require.NoError(t, err)
	assert.Equal(t, types.Ptr(false), voices.Body[0].Recommended)

	// A page at a time, with ElevenLabs' token, and how many there are,
	// which it's asked for
	asked = nil
	voices, err = c.ListVoices(context.Background(), model, llm.WithLimit(1))
	require.NoError(t, err)
	require.Len(t, voices.Body, 1)
	assert.Equal(t, uint(2), voices.Count)
	assert.Equal(t, "page-2", voices.Next)
	voices, err = c.ListVoices(context.Background(), model, llm.WithLimit(1), llm.WithPageToken(voices.Next))
	require.NoError(t, err)
	assert.Equal(t, "abc", voices.Body[0].ID)
	assert.Empty(t, voices.Next)
	assert.Equal(t, []string{
		"/v2/voices?include_total_count=true&page_size=1",
		"/v2/voices?include_total_count=true&next_page_token=page-2&page_size=1",
	}, asked)
	_, err = c.ListVoices(context.Background(), model, llm.WithOpt(llm.VoiceKey, "x"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}

func Test_V2Path(t *testing.T) {
	for endpoint, want := range map[string][]any{
		"https://api.elevenlabs.io/v1":        {"v2", "voices"},
		"https://api.elevenlabs.io/v1/":       {"v2", "voices"},
		"https://proxy.example/elevenlabs/v1": {"elevenlabs", "v2", "voices"},
		"https://proxy.example/elevenlabs":    {"elevenlabs", "v2", "voices"},
	} {
		c, err := New(endpoint, "key")
		require.NoError(t, err)
		assert.Equal(t, want, c.v2Path("voices"), endpoint)
	}
}
