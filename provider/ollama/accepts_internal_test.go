package ollama

import (
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

// Test_Accepts checks the types of attachment the models accept
func Test_Accepts(t *testing.T) {
	c := &Client{}
	assert.True(t, c.Accepts(nil, "image/png"))
	assert.False(t, c.Accepts(nil, "application/pdf"))
	assert.False(t, c.Accepts(nil, "audio/mpeg"))
}
