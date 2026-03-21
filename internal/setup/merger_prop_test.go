package setup_test

// Feature: mnemos-autopilot, Property 17: MCP config merge is idempotent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/setup"
	"pgregory.net/rapid"
)

// TestProp_MCPMergeIdempotent verifies that calling MergeMCPConfig N times never
// creates duplicate "mnemos" entries — exactly one entry must exist after any number of calls.
//
// Validates: Requirements 4.5 (Property 17)
func TestProp_MCPMergeIdempotent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, ".mcp.json")

		// Optionally seed the file with an existing config
		seedType := rapid.IntRange(0, 2).Draw(rt, "seed_type")
		// 0 = no file, 1 = file without mnemos entry, 2 = file with existing mnemos entry
		switch seedType {
		case 1:
			initial := `{"mcpServers":{"other":{"command":"other","args":[]}}}`
			if err := os.WriteFile(filePath, []byte(initial), 0o644); err != nil {
				rt.Fatal(err)
			}
		case 2:
			initial := `{"mcpServers":{"mnemos":{"command":"old","args":["old"]},"other":{"command":"other","args":[]}}}`
			if err := os.WriteFile(filePath, []byte(initial), 0o644); err != nil {
				rt.Fatal(err)
			}
		}

		entry := setup.MnemosMCPEntry{
			Command: "mnemos",
			Args:    []string{"hook"},
		}

		// Call MergeMCPConfig N times (1–10)
		n := rapid.IntRange(1, 10).Draw(rt, "num_calls")
		for i := 0; i < n; i++ {
			if err := setup.MergeMCPConfig(filePath, "mnemos", entry); err != nil {
				rt.Fatalf("MergeMCPConfig call %d failed: %v", i+1, err)
			}
		}

		// Read and parse the result
		data, err := os.ReadFile(filePath)
		if err != nil {
			rt.Fatalf("failed to read result file: %v", err)
		}

		var root map[string]json.RawMessage
		if err := json.Unmarshal(data, &root); err != nil {
			rt.Fatalf("failed to unmarshal result: %v", err)
		}

		rawServers, ok := root["mcpServers"]
		if !ok {
			rt.Fatal("mcpServers key missing from result")
		}

		var servers map[string]json.RawMessage
		if err := json.Unmarshal(rawServers, &servers); err != nil {
			rt.Fatalf("failed to unmarshal mcpServers: %v", err)
		}

		// Count occurrences of "mnemos" key — must be exactly 1
		count := 0
		for k := range servers {
			if k == "mnemos" {
				count++
			}
		}

		if count != 1 {
			rt.Fatalf("expected exactly 1 'mnemos' entry after %d call(s), got %d (seed_type=%d)",
				n, count, seedType)
		}
	})
}
