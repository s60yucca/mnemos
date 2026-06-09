package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigPathFromArgs(t *testing.T) {
	t.Setenv("MNEMOS_CONFIG", "/env/config.yaml")

	assert.Equal(t, "/env/config.yaml", configPathFromArgs(nil))
	assert.Equal(t, "/tmp/config.yaml", configPathFromArgs([]string{"--config", "/tmp/config.yaml", "status"}))
	assert.Equal(t, "/tmp/inline.yaml", configPathFromArgs([]string{"--config=/tmp/inline.yaml", "status"}))
}
