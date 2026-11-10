package ollama_test

import (
	"context"
	"os"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

// ollamaAddr is the address of a live Ollama server, e.g. "localhost:11434" -
// tests which need one are skipped if it's not set.
var ollamaAddr string

func TestMain(m *testing.M) {
	ollamaAddr = os.Getenv("OLLAMA_ADDR")
	os.Exit(m.Run())
}

// requireClient returns a Client pointed at ollamaAddr, or skips the test if
// OLLAMA_ADDR isn't set.
func requireClient(t *testing.T) *ollama.Client {
	t.Helper()
	if ollamaAddr == "" {
		t.Skip("OLLAMA_ADDR not set, skipping")
	}
	c, err := ollama.New(ollamaAddr)
	require.NoError(t, err)
	return c
}

///////////////////////////////////////////////////////////////////////////////
// New

func Test_New_DefaultEndpoint(t *testing.T) {
	c, err := ollama.New("")
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func Test_New_HostPortNoScheme(t *testing.T) {
	// A bare host:port (no scheme) should be accepted and normalised to
	// "http://host:port/api"
	c, err := ollama.New("localhost:11434")
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func Test_New_URLWithNoPath(t *testing.T) {
	// A URL with no path should get "/api" appended
	c, err := ollama.New("http://localhost:11434")
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func Test_URL(t *testing.T) {
	// The endpoint is normalized
	for endpoint, want := range map[string]string{
		"":                           "http://localhost:11434/api",
		"localhost:11434":            "http://localhost:11434/api",
		"nestor":                     "http://nestor:11434/api",
		"nestor:8080":                "http://nestor:8080/api",
		"nestor/":                    "http://nestor:11434/api",
		"192.168.1.2":                "http://192.168.1.2:11434/api",
		"[::1]":                      "http://[::1]:11434/api",
		"http://nestor":              "http://nestor/api",
		"http://localhost:11434":     "http://localhost:11434/api",
		"http://localhost:11434/":    "http://localhost:11434/api",
		"http://localhost:11434/api": "http://localhost:11434/api",
		"https://ollama.example/v2":  "https://ollama.example/v2",
	} {
		c, err := ollama.New(endpoint)
		require.NoError(t, err, "endpoint %q", endpoint)
		require.NotNil(t, c.URL())
		assert.Equal(t, want, c.URL().String(), "endpoint %q", endpoint)
	}
}

func Test_URL_Copy(t *testing.T) {
	// Changing the returned URL doesn't change the client's
	c, err := ollama.New("")
	require.NoError(t, err)
	c.URL().Path = "/changed"
	assert.Equal(t, "/api", c.URL().Path)
}

///////////////////////////////////////////////////////////////////////////////
// Name / Self / interface conformance

func Test_Name(t *testing.T) {
	c, err := ollama.New("")
	require.NoError(t, err)
	assert.Equal(t, "ollama", c.Name())
}

func Test_Self(t *testing.T) {
	c, err := ollama.New("")
	require.NoError(t, err)
	assert.Same(t, c, c.Self())
}

func Test_ImplementsProvider(t *testing.T) {
	assert.Implements(t, (*llm.Provider)(nil), &ollama.Client{})
}

///////////////////////////////////////////////////////////////////////////////
// Ping

func Test_Ping(t *testing.T) {
	c := requireClient(t)
	assert.NoError(t, c.Ping(context.Background()))
}

func Test_Ping_Unreachable(t *testing.T) {
	// A client pointed at a port nothing is listening on should fail to ping,
	// without needing OLLAMA_ADDR to be set
	c, err := ollama.New("http://localhost:1")
	require.NoError(t, err)
	assert.Error(t, c.Ping(context.Background()))
}

func Test_ListVoices_Live(t *testing.T) {
	// The voices of the speech model in OLLAMA_SPEECH_MODEL, or phonon, which
	// Ollama shows, when it's pulled
	c := requireClient(t)
	name := os.Getenv("OLLAMA_SPEECH_MODEL")
	if name == "" {
		name = "phonon:latest"
	}
	model, err := c.GetModel(context.Background(), name)
	if err != nil {
		t.Skipf("%s isn't available: %v", name, err)
	}
	voices, err := c.ListVoices(context.Background(), model)
	require.NoError(t, err)
	require.NotEmpty(t, voices.Body)
	t.Logf("%d voices, such as %v", voices.Count, voices.Body[0])
}
