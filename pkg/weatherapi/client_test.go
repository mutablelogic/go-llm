package weatherapi_test

import (
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	weatherapi "github.com/mutablelogic/go-client/pkg/weatherapi"
	assert "github.com/stretchr/testify/assert"
)

func Test_client_001(t *testing.T) {
	assert := assert.New(t)
	client, err := weatherapi.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)
	t.Log(client)
}

func Test_client_002(t *testing.T) {
	assert := assert.New(t)
	client, err := weatherapi.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	weather, err := client.Current("Berlin, Germany")
	if !assert.NoError(err) {
		t.SkipNow()
	}
	t.Log(weather)
}

func Test_client_003(t *testing.T) {
	assert := assert.New(t)
	client, err := weatherapi.New(GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	forecast, err := client.Forecast("Berlin, Germany", weatherapi.OptDays(2))
	if !assert.NoError(err) {
		t.SkipNow()
	}
	t.Log(forecast)
}

///////////////////////////////////////////////////////////////////////////////
// ENVIRONMENT

func GetApiKey(t *testing.T) string {
	key := os.Getenv("WEATHERAPI_KEY")
	if key == "" {
		t.Skip("WEATHERAPI_KEY not set")
		t.SkipNow()
	}
	return key
}
