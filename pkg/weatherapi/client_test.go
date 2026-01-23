package weatherapi_test

import (
	"flag"
	"log"
	"os"
	"strconv"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	tool "github.com/mutablelogic/go-llm/pkg/tool"
	weatherapi "github.com/mutablelogic/go-llm/pkg/weatherapi"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// TEST SET-UP

var (
	client *weatherapi.Client
	tools  []tool.Tool
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
	api_key := os.Getenv("WEATHER_API_KEY")
	if api_key == "" {
		log.Print("WEATHER_API_KEY not set")
		os.Exit(0)
	}

	// Create client
	var err error
	client, err = weatherapi.New(api_key, opts.OptTrace(os.Stderr, verbose))
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}

	// Create tools
	tools, err = weatherapi.NewTools(api_key, opts.OptTrace(os.Stderr, verbose))
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

func Test_client_002(t *testing.T) {
	assert := assert.New(t)

	weather, err := client.Current(t.Context(), &weatherapi.CurrentWeatherRequest{Query: "Berlin, Germany"})
	if !assert.NoError(err) {
		t.SkipNow()
	}
	t.Log(weather)
}

func Test_client_003(t *testing.T) {
	assert := assert.New(t)

	forecast, err := client.Forecast(t.Context(), &weatherapi.ForecastWeatherRequest{Query: "Berlin, Germany", Days: 2})
	if !assert.NoError(err) {
		t.SkipNow()
	}
	t.Log(forecast)
}

func Test_tools_001(t *testing.T) {
	assert := assert.New(t)
	assert.Len(tools, 3)
	t.Log("Tools:", len(tools))
}
