//go:build benchmark

package benchmark

import (
	"bytes"
	"strings"
	"testing"
)

func TestBenchmarkCmdRun(t *testing.T) {
	cmd := NewBenchmarkCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "┌") {
		t.Errorf("expected output to contain ┌, got: %s", buf.String())
	}
}

func TestBenchmarkCmdRunUnknownScenario(t *testing.T) {
	cmd := NewBenchmarkCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"run", "--scenario", "unknown"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for unknown scenario, got nil")
	}
}
