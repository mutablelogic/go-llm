package main

import (
	"testing"

	// Packages
	kong "github.com/alecthomas/kong"
	require "github.com/stretchr/testify/require"
)

func Test_CLI(t *testing.T) {
	// The commands, and their flags, can be parsed, such as an enum, which
	// needs a default, or to be required, which kong otherwise panics on, as
	// the command starts
	_, err := kong.New(&CLI{}, kong.Name("llm"))
	require.NoError(t, err)
}
