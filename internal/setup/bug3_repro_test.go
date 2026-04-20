package setup_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/setup"
)

// TestBug3_EnvKeyStripped_Reproduction manually reproduces Bug 3:
// MergeMCPConfig strips the "env" key from an existing mnemos entry.
//
// Input .mcp.json:
//
//	{ "mcpServers": { "mnemos": { "command": "mnemos", "args": ["serve"], "env": { "MNEMOS_PROJECT_ID": "sdk-dev" } } } }
//
// Expected after MergeMCPConfig: env key IS still present
// Actual (unfixed): env key is GONE
func TestBug3_EnvKeyStripped_Reproduction(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, ".mcp.json")

	// Input: .mcp.json with env key set
	input := `{
  "mcpServers": {
    "mnemos": {
      "command": "mnemos",
      "args": ["serve"],
      "env": { "MNEMOS_PROJECT_ID": "sdk-dev" }
    }
  }
}`
	if err := os.WriteFile(filePath, []byte(input), 0o644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	// Call MergeMCPConfig (the unfixed version)
	entry := setup.MnemosMCPEntry{
		Command: "mnemos",
		Args:    []string{"serve"},
	}
	if err := setup.MergeMCPConfig(filePath, "mnemos", entry); err != nil {
		t.Fatalf("MergeMCPConfig failed: %v", err)
	}

	// Read back the result
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	t.Logf("Output .mcp.json:\n%s", string(data))

	// Parse and check for env key
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(root["mcpServers"], &servers); err != nil {
		t.Fatalf("failed to unmarshal mcpServers: %v", err)
	}

	var mnemos map[string]json.RawMessage
	if err := json.Unmarshal(servers["mnemos"], &mnemos); err != nil {
		t.Fatalf("failed to unmarshal mnemos entry: %v", err)
	}

	// This assertion FAILS on unfixed code — env key is stripped
	if _, ok := mnemos["env"]; !ok {
		t.Errorf("BUG CONFIRMED: env key was stripped from mnemos entry\n"+
			"Input:  %s\n"+
			"Output: %s\n"+
			"Counterexample: input had MNEMOS_PROJECT_ID in env, output has no env key at all",
			input, string(data))
	} else {
		t.Logf("env key preserved: %s", string(mnemos["env"]))
	}
}
