package mistral_test

import (
	"flag"
	"log"
	"os"
	"strconv"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	mistral "github.com/mutablelogic/go-llm/pkg/mistral"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

var (
	client *mistral.Client
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

	// API KEY
	api_key := os.Getenv("MISTRAL_API_KEY")
	if api_key == "" {
		log.Print("MISTRAL_API_KEY not set")
		os.Exit(0)
	}

	// Create client
	var err error
	client, err = mistral.New(api_key, opts.OptTrace(os.Stderr, verbose))
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
