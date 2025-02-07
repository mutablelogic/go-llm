package ollama_test

import (
	"flag"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	// Packages
	opts "github.com/mutablelogic/go-client"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

var (
	client *ollama.Client
)

func TestMain(m *testing.M) {
	var verbose bool

	// Verbose output
	flag.Parse()
	if f := flag.Lookup("test.v"); f != nil {
		if v, err := strconv.ParseBool(f.Value.String()); err == nil {
			verbose = v
		}
	}

	// Endpoint
	endpoint_url := os.Getenv("OLLAMA_URL")
	if endpoint_url == "" {
		log.Print("OLLAMA_URL not set")
		os.Exit(0)
	}

	// Create client
	var err error
	client, err = ollama.New(endpoint_url, opts.OptTrace(os.Stderr, verbose), opts.OptTimeout(5*time.Minute))
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
	os.Exit(m.Run())
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func Test_client_001(t *testing.T) {
	assert := assert.New(t)
	assert.NotNil(client)
	t.Log(client)
}
