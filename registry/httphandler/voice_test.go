package httphandler_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ListVoices(t *testing.T) {
	// An Ollama server with a speech model, which has thirty voices, and
	// eliza, which doesn't speak
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/tags":
			_, _ = io.WriteString(w, `{"models":[{"name":"phonon:latest"}]}`)
		case r.URL.Path == "/api/show" && strings.Contains(string(body), "phonon"):
			voices := make([]string, 30)
			for i := range voices {
				voices[i] = fmt.Sprintf("%q", fmt.Sprintf("Voice %d", i))
			}
			_, _ = io.WriteString(w, `{"capabilities":["speech"],"voices":[`+strings.Join(voices, ",")+`]}`)
		case r.URL.Path == "/api/version":
			_, _ = io.WriteString(w, `{"version":"0.20.0"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(stub.Close)
	speech, err := ollama.New(stub.URL)
	require.NoError(t, err)
	chat, err := eliza.New()
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{speech, chat}))

	// Every voice, with the model, by its canonical name
	var list schema.VoiceList
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model/phonon:latest/voice", nil, &list))
	assert.Equal(t, "ollama:phonon:latest", list.Model)
	assert.Len(t, list.Body, 30)
	assert.Empty(t, list.Next)

	// A page at a time
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model/phonon:latest/voice?limit=10", nil, &list))
	require.Len(t, list.Body, 10)
	assert.Equal(t, "10", list.Next)
	next := list.Next
	list = schema.VoiceList{}
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model/phonon:latest/voice?limit=25&next="+next, nil, &list))
	assert.Len(t, list.Body, 20)
	assert.Equal(t, "Voice 10", list.Body[0].ID)
	assert.Empty(t, list.Next)

	// Those like a name, a page at a time
	list = schema.VoiceList{}
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model/phonon:latest/voice?voice=VOICE+2", nil, &list))
	assert.Len(t, list.Body, 11, "Voice 2, and Voice 20 to 29")
	assert.Equal(t, "VOICE 2", list.Voice)
	list = schema.VoiceList{}
	require.Equal(t, http.StatusOK, do(t, server, http.MethodGet, "llm/model/phonon:latest/voice?voice=voice+2&limit=5", nil, &list))
	assert.Len(t, list.Body, 5)
	assert.Equal(t, "5", list.Next)

	// A token which isn't one, a model which doesn't speak, and one which
	// isn't there
	assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodGet, "llm/model/phonon:latest/voice?next=nonsense", nil, nil))
	assert.Equal(t, http.StatusNotImplemented, do(t, server, http.MethodGet, "llm/model/"+eliza.ModelName+"/voice", nil, nil))
	assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodGet, "llm/model/missing/voice", nil, nil))
}
