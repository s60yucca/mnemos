package setup_test

// Preservation Property Tests — Task 2 of setup-claude-global-fix bugfix spec.
//
// These tests assert BASELINE behavior that must be PRESERVED after the fix.
// They should ALL PASS on unfixed code. If any fail, it means the baseline
// behavior is already broken (not caused by our fix).
//
// Run with: go test ./internal/setup/... -run TestPreservation -v
//
// P2a — Other clients unaffected: MergeMCPConfig is client-agnostic; works correctly
//        for non-claude MCP paths (kiro, cursor, gemini-cli).
// P2b — Local MCP idempotency: double-merge = single-merge for any valid JSON input.
// P2c — Hook idempotency: MergeClaudeSettings never removes non-mnemos hooks and
//        never duplicates mnemos hooks.
// P2d — Non-TTY scope: resolveScope not yet implemented — skipped (placeholder).
// P2e — Explicit flag override: resolveScope not yet implemented — skipped (placeholder).
// P2f — Linux/Windows launchd skip: InstallLaunchdPlist not yet implemented — skipped (placeholder).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/setup"
	"pgregory.net/rapid"
)

// callMergeClaudeSettings is a stable wrapper around setup.MergeClaudeSettings.
// It passes a fixed test binPath so the property test is independent of the
// actual binary location on the test machine.
// Updated in task 3.3 after the signature change.
func callMergeClaudeSettings(path string) error {
	return setup.MergeClaudeSettings(path, "/usr/local/bin/mnemos")
}

// TestPreservation is the top-level suite for all six preservation property tests.
func TestPreservation(t *testing.T) {
	t.Run("P2a_OtherClientsUnaffected", testP2aOtherClientsUnaffected)
	t.Run("P2b_LocalMCPIdempotency", testP2bLocalMCPIdempotency)
	t.Run("P2c_HookIdempotency", testP2cHookIdempotency)
	t.Run("P2d_NonTTYScope", testP2dNonTTYScope)
	t.Run("P2e_ExplicitFlagOverride", testP2eExplicitFlagOverride)
	t.Run("P2f_LinuxWindowsLaunchdSkip", testP2fLinuxWindowsLaunchdSkip)
}

// testP2aOtherClientsUnaffected verifies that MergeMCPConfig is client-agnostic.
// For non-claude clients (kiro, cursor, gemini-cli), calling MergeMCPConfig on their
// MCP paths works correctly: idempotent, preserves other keys, adds mnemos entry.
//
// This confirms the merge function itself has no claude-specific behavior that could
// break other clients after the fix.
//
// Validates: Requirements 3.5
func testP2aOtherClientsUnaffected(t *testing.T) {
	// Non-claude client MCP paths from clients.go
	nonClaudeClients := []struct {
		name    string
		mcpPath string
	}{
		{"kiro", ".kiro/settings/mcp.json"},
		{"cursor", ".mcp.json"},
		{"gemini-cli", ".gemini/settings.json"},
	}

	for _, client := range nonClaudeClients {
		client := client
		t.Run(client.name, func(t *testing.T) {
			dir := t.TempDir()
			mcpFile := filepath.Join(dir, client.mcpPath)

			// Ensure parent directory exists (as setup would do)
			if err := os.MkdirAll(filepath.Dir(mcpFile), 0o755); err != nil {
				t.Fatalf("failed to create dir: %v", err)
			}

			// Seed with an existing config containing a non-mnemos server
			existing := `{"mcpServers":{"other-tool":{"command":"other","args":["--flag"]}}}`
			if err := os.WriteFile(mcpFile, []byte(existing), 0o644); err != nil {
				t.Fatalf("failed to write seed file: %v", err)
			}

			entry := setup.MnemosMCPEntry{
				Command: "mnemos",
				Args:    []string{"serve"},
			}

			// Call MergeMCPConfig twice (idempotency check)
			if err := setup.MergeMCPConfig(mcpFile, "mnemos", entry); err != nil {
				t.Fatalf("first MergeMCPConfig failed for %s: %v", client.name, err)
			}
			if err := setup.MergeMCPConfig(mcpFile, "mnemos", entry); err != nil {
				t.Fatalf("second MergeMCPConfig failed for %s: %v", client.name, err)
			}

			// Read back and verify
			data, err := os.ReadFile(mcpFile)
			if err != nil {
				t.Fatalf("failed to read result: %v", err)
			}

			var root map[string]json.RawMessage
			if err := json.Unmarshal(data, &root); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			var servers map[string]json.RawMessage
			if err := json.Unmarshal(root["mcpServers"], &servers); err != nil {
				t.Fatalf("failed to unmarshal mcpServers: %v", err)
			}

			// mnemos entry must be present
			if _, ok := servers["mnemos"]; !ok {
				t.Errorf("client %s: mnemos entry missing after MergeMCPConfig", client.name)
			}

			// other-tool entry must be preserved
			if _, ok := servers["other-tool"]; !ok {
				t.Errorf("client %s: other-tool entry was removed by MergeMCPConfig", client.name)
			}

			// Exactly two entries — no duplicates
			if len(servers) != 2 {
				t.Errorf("client %s: expected 2 server entries, got %d", client.name, len(servers))
			}
		})
	}
}

// testP2bLocalMCPIdempotency verifies that for any valid JSON input,
// calling MergeMCPConfig twice produces the same result as calling it once.
//
// Property: double-merge = single-merge for all valid JSON inputs.
// Specifically: exactly one mnemos entry, all original keys preserved.
//
// Validates: Requirements 3.3, 3.4
func testP2bLocalMCPIdempotency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, ".mcp.json")

		// Generate a random set of extra server names (non-mnemos)
		numExtra := rapid.IntRange(0, 5).Draw(rt, "num_extra_servers")
		extraServers := make(map[string]interface{})
		for i := 0; i < numExtra; i++ {
			// Use deterministic names to avoid JSON key collisions
			name := rapid.StringMatching(`[a-z][a-z0-9-]{0,10}`).Draw(rt, "server_name")
			if name == "mnemos" {
				name = "other-" + name
			}
			extraServers[name] = map[string]interface{}{
				"command": "tool-" + name,
				"args":    []string{"--run"},
			}
		}

		// Generate random extra top-level keys
		numTopKeys := rapid.IntRange(0, 3).Draw(rt, "num_top_keys")
		topLevel := map[string]interface{}{
			"mcpServers": extraServers,
		}
		for i := 0; i < numTopKeys; i++ {
			key := rapid.StringMatching(`[a-z][a-z0-9]{2,8}`).Draw(rt, "top_key")
			if key != "mcpServers" {
				topLevel[key] = rapid.StringMatching(`[a-zA-Z0-9 ]{1,20}`).Draw(rt, "top_val")
			}
		}

		// Write the seed file
		seedBytes, err := json.Marshal(topLevel)
		if err != nil {
			rt.Fatalf("failed to marshal seed: %v", err)
		}
		if err := os.WriteFile(filePath, seedBytes, 0o644); err != nil {
			rt.Fatalf("failed to write seed file: %v", err)
		}

		entry := setup.MnemosMCPEntry{
			Command: "mnemos",
			Args:    []string{"serve"},
		}

		// First merge
		if err := setup.MergeMCPConfig(filePath, "mnemos", entry); err != nil {
			rt.Fatalf("first MergeMCPConfig failed: %v", err)
		}
		afterFirst, err := os.ReadFile(filePath)
		if err != nil {
			rt.Fatalf("failed to read after first merge: %v", err)
		}

		// Second merge (idempotency)
		if err := setup.MergeMCPConfig(filePath, "mnemos", entry); err != nil {
			rt.Fatalf("second MergeMCPConfig failed: %v", err)
		}
		afterSecond, err := os.ReadFile(filePath)
		if err != nil {
			rt.Fatalf("failed to read after second merge: %v", err)
		}

		// Parse both results
		var root1, root2 map[string]json.RawMessage
		if err := json.Unmarshal(afterFirst, &root1); err != nil {
			rt.Fatalf("failed to unmarshal after first: %v", err)
		}
		if err := json.Unmarshal(afterSecond, &root2); err != nil {
			rt.Fatalf("failed to unmarshal after second: %v", err)
		}

		// Property 1: exactly one mnemos entry after both merges
		var servers1, servers2 map[string]json.RawMessage
		if err := json.Unmarshal(root1["mcpServers"], &servers1); err != nil {
			rt.Fatalf("failed to unmarshal mcpServers after first: %v", err)
		}
		if err := json.Unmarshal(root2["mcpServers"], &servers2); err != nil {
			rt.Fatalf("failed to unmarshal mcpServers after second: %v", err)
		}

		if _, ok := servers1["mnemos"]; !ok {
			rt.Fatal("mnemos entry missing after first merge")
		}
		if _, ok := servers2["mnemos"]; !ok {
			rt.Fatal("mnemos entry missing after second merge")
		}

		// Property 2: same number of server entries (no duplicates created)
		if len(servers1) != len(servers2) {
			rt.Fatalf("server count changed between first (%d) and second (%d) merge",
				len(servers1), len(servers2))
		}

		// Property 3: all original extra server keys preserved after both merges
		for name := range extraServers {
			if _, ok := servers1[name]; !ok {
				rt.Fatalf("extra server %q lost after first merge", name)
			}
			if _, ok := servers2[name]; !ok {
				rt.Fatalf("extra server %q lost after second merge", name)
			}
		}

		// Property 4: all original top-level keys preserved
		for key := range topLevel {
			if _, ok := root1[key]; !ok {
				rt.Fatalf("top-level key %q lost after first merge", key)
			}
			if _, ok := root2[key]; !ok {
				rt.Fatalf("top-level key %q lost after second merge", key)
			}
		}
	})
}

// testP2cHookIdempotency verifies that for any settings.json with non-mnemos hooks,
// calling MergeClaudeSettings twice:
//   - never removes non-mnemos hooks
//   - never creates duplicate mnemos hook entries
//   - results in exactly one mnemos entry per event
//
// Uses callMergeClaudeSettings wrapper which will be updated in task 3.12 to pass binPath.
//
// Validates: Requirements 3.1
func testP2cHookIdempotency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "settings.json")

		// Generate random non-mnemos hook commands
		numHooks := rapid.IntRange(0, 4).Draw(rt, "num_existing_hooks")

		type hookEntry struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		}
		type hookGroup struct {
			Hooks []hookEntry `json:"hooks"`
		}

		events := []string{"SessionStart", "UserPromptSubmit", "SessionEnd"}
		hooks := make(map[string][]hookGroup)

		for i := 0; i < numHooks; i++ {
			event := events[rapid.IntRange(0, len(events)-1).Draw(rt, "event")]
			cmd := rapid.StringMatching(`[a-z][a-z0-9-]{2,15}`).Draw(rt, "hook_cmd")
			// Ensure it's not a mnemos hook command
			if strings.HasPrefix(cmd, "mnemos") {
				cmd = "custom-" + cmd
			}
			hooks[event] = append(hooks[event], hookGroup{
				Hooks: []hookEntry{{Type: "command", Command: cmd}},
			})
		}

		// Write seed settings.json
		root := map[string]interface{}{"hooks": hooks}
		seedBytes, err := json.Marshal(root)
		if err != nil {
			rt.Fatalf("failed to marshal seed: %v", err)
		}
		if err := os.WriteFile(filePath, seedBytes, 0o644); err != nil {
			rt.Fatalf("failed to write seed: %v", err)
		}

		// First call
		if err := callMergeClaudeSettings(filePath); err != nil {
			rt.Fatalf("first callMergeClaudeSettings failed: %v", err)
		}

		// Second call (idempotency)
		if err := callMergeClaudeSettings(filePath); err != nil {
			rt.Fatalf("second callMergeClaudeSettings failed: %v", err)
		}

		// Read and parse result
		data, err := os.ReadFile(filePath)
		if err != nil {
			rt.Fatalf("failed to read result: %v", err)
		}

		var resultRoot map[string]json.RawMessage
		if err := json.Unmarshal(data, &resultRoot); err != nil {
			rt.Fatalf("failed to unmarshal result: %v", err)
		}

		var resultHooks map[string]json.RawMessage
		if err := json.Unmarshal(resultRoot["hooks"], &resultHooks); err != nil {
			rt.Fatalf("failed to unmarshal hooks: %v", err)
		}

		type resultHookEntry struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		}
		type resultHookGroup struct {
			Hooks []resultHookEntry `json:"hooks"`
		}

		// Property 1: exactly one mnemos entry per event (no duplicates)
		mnemosSuffixes := map[string]string{
			"SessionStart":     "hook session-start",
			"UserPromptSubmit": "hook prompt-submit",
			"SessionEnd":       "hook session-end",
		}
		for event, suffix := range mnemosSuffixes {
			raw, ok := resultHooks[event]
			if !ok {
				rt.Fatalf("event %q missing from hooks after merge", event)
			}
			var groups []resultHookGroup
			if err := json.Unmarshal(raw, &groups); err != nil {
				rt.Fatalf("failed to unmarshal groups for %q: %v", event, err)
			}

			mnemosCmdCount := 0
			for _, g := range groups {
				for _, h := range g.Hooks {
					if strings.Contains(h.Command, suffix) {
						mnemosCmdCount++
					}
				}
			}
			if mnemosCmdCount != 1 {
				rt.Fatalf("event %q: expected exactly 1 mnemos hook entry (containing %q), got %d (after 2 calls)",
					event, suffix, mnemosCmdCount)
			}
		}

		// Property 2: all original non-mnemos hooks are preserved
		for event, originalGroups := range hooks {
			raw, ok := resultHooks[event]
			if !ok {
				rt.Fatalf("event %q was removed from hooks", event)
			}
			var resultGroups []resultHookGroup
			if err := json.Unmarshal(raw, &resultGroups); err != nil {
				rt.Fatalf("failed to unmarshal result groups for %q: %v", event, err)
			}

			// Collect all commands in result
			resultCmds := make(map[string]bool)
			for _, g := range resultGroups {
				for _, h := range g.Hooks {
					resultCmds[h.Command] = true
				}
			}

			// Every original non-mnemos command must still be present
			for _, g := range originalGroups {
				for _, h := range g.Hooks {
					if !strings.HasPrefix(h.Command, "mnemos") {
						if !resultCmds[h.Command] {
							rt.Fatalf("event %q: non-mnemos hook %q was removed", event, h.Command)
						}
					}
				}
			}
		}
	})
}

// testP2dNonTTYScope documents the expected behavior of resolveScope(false, false)
// when stdout is not a TTY: should return false (local scope) with no output.
//
// resolveScope does not exist yet (it will be added to cmd/mnemos/setup.go in task 3.10
// and is not exported from internal/setup). This test is a placeholder that will be
// updated in task 3.12 once resolveScope is implemented and accessible.
//
// Validates: Requirements 2.9, 3.8
func testP2dNonTTYScope(t *testing.T) {
	t.Skip("resolveScope not yet implemented — placeholder to be updated in task 3.12")
	// When implemented, this test should:
	// 1. Redirect stdout to a pipe (non-TTY)
	// 2. Call resolveScope(false, false)
	// 3. Assert returns false (local scope)
	// 4. Assert no bytes written to stdout
}

// testP2eExplicitFlagOverride documents the expected behavior of resolveScope
// when explicit --global or --local flags are passed: should return the correct
// scope regardless of TTY state.
//
// resolveScope does not exist yet (it will be added to cmd/mnemos/setup.go in task 3.10
// and is not exported from internal/setup). This test is a placeholder that will be
// updated in task 3.12 once resolveScope is implemented and accessible.
//
// Validates: Requirements 3.8
func testP2eExplicitFlagOverride(t *testing.T) {
	t.Skip("resolveScope not yet implemented — placeholder to be updated in task 3.12")
	// When implemented, this test should:
	// 1. Call resolveScope(true, false) — assert returns true (global)
	// 2. Call resolveScope(false, true) — assert returns false (local)
	// Both should work regardless of whether stdout is a TTY.
}

// testP2fLinuxWindowsLaunchdSkip verifies that on non-darwin platforms,
// InstallLaunchdPlist returns nil immediately with no side effects.
//
// On darwin, this test is skipped since the darwin implementation involves
// real launchctl calls that are non-trivial to test in a unit context.
//
// Validates: Requirements 3.6, 3.7
func testP2fLinuxWindowsLaunchdSkip(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("launchd skip test only runs on non-darwin platforms")
	}
	err := setup.InstallLaunchdPlist("/usr/local/bin/mnemos")
	if err != nil {
		t.Errorf("InstallLaunchdPlist should return nil on non-darwin, got: %v", err)
	}
}
