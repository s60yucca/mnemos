package main

import (
	"bytes"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusReportsAutomaticKnowledgeLoopSettings(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDir = "/tmp/mnemos-status"
	cfg.Hook.Enabled = true
	cfg.Autopilot.AutoCompileEnabled = true
	cfg.Autopilot.MinAutoCompileSources = 7

	var output bytes.Buffer
	cmd := newStatusCmd(cfg)
	cmd.SetOut(&output)
	require.NoError(t, cmd.Execute())

	assert.Contains(t, output.String(), "auto_compile: true")
	assert.Contains(t, output.String(), "auto_inject: true")
	assert.Contains(t, output.String(), "min_auto_compile_sources: 7")
}
