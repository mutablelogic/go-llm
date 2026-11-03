package cmd

import (
	"testing"
	"time"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func Test_Timestamp(t *testing.T) {
	// Minutes and seconds, to a tenth of a second, with hours when there
	// are any
	assert.Equal(t, "0:00.0", timestamp(0))
	assert.Equal(t, "0:01.3", timestamp(1300*time.Millisecond))
	assert.Equal(t, "1:05.3", timestamp(65*time.Second+260*time.Millisecond))
	assert.Equal(t, "1:00:00.0", timestamp(time.Hour))
	assert.Equal(t, "2:03:04.5", timestamp(2*time.Hour+3*time.Minute+4500*time.Millisecond))
}
