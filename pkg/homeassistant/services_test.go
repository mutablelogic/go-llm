package homeassistant_test

import (
	"context"
	"os"
	"testing"

	// Packages
	opts "github.com/mutablelogic/go-client"
	homeassistant "github.com/mutablelogic/go-llm/pkg/homeassistant"
	assert "github.com/stretchr/testify/assert"
)

func Test_services_001(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)
	assert.NotNil(client)

	domains, err := client.Domains(context.Background())
	if !assert.NoError(err) {
		t.FailNow()
	}
	assert.NotNil(domains)
	t.Log(domains)
}

// Test calling a service with arbitrary data
func Test_services_002(t *testing.T) {
	assert := assert.New(t)
	client, err := homeassistant.New(GetEndPoint(t), GetApiKey(t), opts.OptTrace(os.Stderr, true))
	assert.NoError(err)

	// List services for homeassistant domain
	services, err := client.Services(context.Background(), "homeassistant")
	assert.NoError(err)
	assert.NotNil(services)
	t.Log("homeassistant services:", len(services))
}
