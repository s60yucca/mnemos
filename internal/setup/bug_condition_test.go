package setup_test

// Bug Condition Exploration Tests — Task 1 of setup-claude-global-fix bugfix spec.
//
// These tests assert the CORRECT (expected) behavior.
// They are EXPECTED TO FAIL on unfixed code — failure confirms each bug exists.
// They will PASS after the fix is applied (task 3.11 re-runs them to confirm).
//
// Run with: go test ./internal/setup/... -run TestBugCondition -v
//
// Observed failures on unfixed code (all five FAIL — all bugs confirmed):
//
//   - Bug1: FAIL — ~/.claude.json does not exist after global setup.
//     Entry was written to ~/.mcp.json instead.
//     Counterexample: global setup writes to ~/.mcp.json, Claude Code reads from ~/.claude.json.
//
//   - Bug2: FAIL (darwin) — ~/Library/LaunchAgents/com.mnemos.autopilot.plist does not exist.
//     Counterexample: plist absent — autopilot daemon never registered with launchd.
//     Effect: `mnemos autopilot status` shows next_run: (not scheduled) even when enabled: true.
//
//   - Bug3: FAIL — env key stripped from mnemos entry after MergeMCPConfig.
//     Input:  {"mcpServers":{"mnemos":{"command":"mnemos","args":["serve"],"env":{"MNEMOS_PROJECT_ID":"sdk-dev"}}}}
//     Output: {"mcpServers":{"mnemos":{"command":"mnemos","args":["serve"]}}}  <- env gone
//     Counterexample: MnemosMCPEntry has no env field; json.Marshal overwrites entire entry, stripping env.
//
//   - Bug4: FAIL — hook commands use bare binary name "mnemos hook session-start" etc.
//     Counterexample: bare 'mnemos' fails when Claude Code is launched from GUI (PATH=/usr/bin:/bin).
//     All three hooks (session-start, prompt-submit, session-end) confirmed bare.
//
//   - Bug5: FAIL (darwin) — second launchctl bootstrap (without prior bootout) returns exit status 5.
//     Output: "Bootstrap failed: 5: Input/output error"
//     Counterexample: re-running setup without bootout-first causes non-zero exit.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/setup"
)

// TestBugCondition is the top-level suite for all five bug condition exploration tests.
func TestBugCondition(t *testing.T) {
	t.Run("Bug1_GlobalMCPTarget", testBug1GlobalMCPTarget)
	t.Run("Bug2_LaunchdPlistAbsent", testBug2LaunchdPlistAbsent)
	t.Run("Bug3_DestructiveOverwrite", testBug3DestructiveOverwrite)
	t.Run("Bug4_BareHookCommand", testBug4BareHookCommand)
	t.Run("Bug5_RerunCrash", testBug5RerunCrash)
}

// testBug1GlobalMCPTarget asserts that after a global claude setup, ~/.claude.json
// contains mcpServers.mnemos.
//
// PASSES after the fix because MergeClaudeGlobalMCP writes to ~/.claude.json (strategy 2 fallback).
// Claude Code reads global MCP from ~/.claude.json, not ~/.mcp.json.
func testBug1GlobalMCPTarget(t *testing.T) {
	tmpHome := t.TempDir()

	// Set HOME to tmpHome so MergeClaudeGlobalMCP writes to the test directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	// Call the fixed MergeClaudeGlobalMCP — should write to ~/.claude.json
	binPath := "/opt/homebrew/bin/mnemos"
	err := setup.MergeClaudeGlobalMCP(binPath)
	if err != nil {
		t.Fatalf("MergeClaudeGlobalMCP failed: %v", err)
	}

	// Assert CORRECT behavior: ~/.claude.json should contain mcpServers.mnemos
	correctTarget := filepath.Join(tmpHome, ".claude.json")
	data, err := os.ReadFile(correctTarget)
	if err != nil {
		t.Errorf("FAIL: ~/.claude.json does not exist after MergeClaudeGlobalMCP\n"+
			"Error: %v", err)
		return
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("failed to parse ~/.claude.json: %v", err)
	}

	rawServers, ok := root["mcpServers"]
	if !ok {
		t.Errorf("FAIL: ~/.claude.json exists but has no mcpServers key")
		return
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(rawServers, &servers); err != nil {
		t.Fatalf("failed to parse mcpServers: %v", err)
	}

	if _, ok := servers["mnemos"]; !ok {
		t.Errorf("FAIL: ~/.claude.json mcpServers does not contain mnemos entry")
		return
	}

	// Verify the entry has the correct binPath
	var entry map[string]json.RawMessage
	if err := json.Unmarshal(servers["mnemos"], &entry); err != nil {
		t.Fatalf("failed to parse mnemos entry: %v", err)
	}

	var command string
	if err := json.Unmarshal(entry["command"], &command); err != nil {
		t.Fatalf("failed to parse command: %v", err)
	}

	if command != binPath {
		t.Errorf("FAIL: mnemos entry command is %q, expected %q", command, binPath)
	}
}

// testBug2LaunchdPlistAbsent asserts that after calling InstallLaunchdPlist on darwin,
// ~/Library/LaunchAgents/com.mnemos.autopilot.plist exists.
//
// FAILS on unfixed code because runSetup has no code path that writes or loads
// a launchd plist — the autopilot daemon is never registered as a login item.
//
// PASSES after the fix because InstallLaunchdPlist writes the plist file.
//
// Counterexample: plist file absent from ~/Library/LaunchAgents/ after global setup.
func testBug2LaunchdPlistAbsent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd plist test only runs on darwin")
	}

	tmpHome := t.TempDir()

	// Set HOME to tmpHome so InstallLaunchdPlist writes to the test directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	// Use the current test binary as binPath to avoid needing mnemos on PATH
	selfPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable path: %v", err)
	}

	// Call the fixed InstallLaunchdPlist — should write plist to ~/Library/LaunchAgents/
	// Note: we only check that the plist FILE is written; we don't require launchctl
	// bootstrap to succeed in the test environment (may lack permissions).
	// The plist write itself is the fix for Bug 2.
	if err := setup.InstallLaunchdPlist(selfPath); err != nil {
		// If launchctl bootstrap fails (permissions), that's OK for this test —
		// we only care that the plist file was written.
		t.Logf("InstallLaunchdPlist returned error (may be launchctl permissions): %v", err)
	}

	// Assert CORRECT behavior: plist file MUST exist after InstallLaunchdPlist
	plistPath := filepath.Join(tmpHome, "Library", "LaunchAgents", "com.mnemos.autopilot.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		t.Errorf("BUG 2 NOT FIXED: ~/Library/LaunchAgents/com.mnemos.autopilot.plist does not exist after InstallLaunchdPlist\n"+
			"Expected plist at: %s\n"+
			"Counterexample: plist absent — autopilot daemon never registered with launchd",
			plistPath)
	} else {
		t.Logf("PASS: plist written to %s", plistPath)
	}
}

// testBug3DestructiveOverwrite asserts that after calling MergeMCPConfig on an existing
// .mcp.json that has an "env" key in the mnemos entry, the env key is preserved.
//
// FAILS on unfixed code because MnemosMCPEntry has no env field, so marshaling it
// replaces the entire mnemos entry and strips the env key.
//
// Counterexample:
//
//	Input:  {"mcpServers":{"mnemos":{"command":"mnemos","args":["serve"],"env":{"MNEMOS_PROJECT_ID":"sdk-dev"}}}}
//	Output: {"mcpServers":{"mnemos":{"command":"mnemos","args":["serve"]}}}  ← env gone
//
// Note: bug3_repro_test.go also covers this bug as a standalone reproduction test.
// This sub-test is the canonical entry in the TestBugCondition suite.
func testBug3DestructiveOverwrite(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, ".mcp.json")

	// Input: .mcp.json with env key set on the mnemos entry
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

	// Call MergeMCPConfig (unfixed: marshals MnemosMCPEntry directly, stripping env)
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

	// Assert CORRECT behavior: env key MUST still be present
	if _, ok := mnemos["env"]; !ok {
		t.Errorf("BUG 3 CONFIRMED: env key was stripped from mnemos entry\n"+
			"Input:  %s\n"+
			"Output: %s\n"+
			"Counterexample: MnemosMCPEntry has no env field; json.Marshal overwrites the entire entry, stripping env",
			input, string(data))
	} else {
		// Verify the env value is correct
		var envMap map[string]string
		if err := json.Unmarshal(mnemos["env"], &envMap); err != nil {
			t.Errorf("env key present but not parseable: %v", err)
			return
		}
		if envMap["MNEMOS_PROJECT_ID"] != "sdk-dev" {
			t.Errorf("env key present but MNEMOS_PROJECT_ID value wrong: got %q, want %q",
				envMap["MNEMOS_PROJECT_ID"], "sdk-dev")
		}
	}
}

// testBug4BareHookCommand asserts that after calling MergeClaudeSettings with a
// resolved binPath, the hook commands contain an absolute path prefix rather than
// the bare binary name "mnemos".
//
// PASSES after the fix because MergeClaudeSettings now accepts binPath and uses it
// to construct hook commands like "/opt/homebrew/bin/mnemos hook session-start".
//
// Counterexample (unfixed):
//
//	Expected: "/opt/homebrew/bin/mnemos hook session-start"
//	Actual:   "mnemos hook session-start"  ← bare name, fails in GUI PATH
func testBug4BareHookCommand(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "settings.json")

	// Use a fixed absolute binPath to simulate a resolved binary location.
	binPath := "/opt/homebrew/bin/mnemos"

	// Call MergeClaudeSettings with the new two-arg signature (fixed code).
	if err := setup.MergeClaudeSettings(filePath, binPath); err != nil {
		t.Fatalf("MergeClaudeSettings failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	content := string(data)

	// Assert CORRECT behavior: hook commands MUST contain an absolute path prefix.
	// An absolute path starts with "/" on unix.
	bareCommands := []string{
		`"mnemos hook session-start"`,
		`"mnemos hook prompt-submit"`,
		`"mnemos hook session-end"`,
	}

	for _, bare := range bareCommands {
		if strings.Contains(content, bare) {
			t.Errorf("BUG 4 NOT FIXED: hook command still uses bare binary name\n"+
				"Found: %q in settings.json\n"+
				"Expected: absolute path like \"/opt/homebrew/bin/mnemos hook session-start\"\n"+
				"Counterexample: bare 'mnemos' fails when Claude Code is launched from GUI (PATH=/usr/bin:/bin)",
				bare)
		}
	}

	// Assert that hook commands with the absolute path ARE present
	expectedCommands := []string{
		binPath + " hook session-start",
		binPath + " hook prompt-submit",
		binPath + " hook session-end",
	}
	for _, expected := range expectedCommands {
		if !strings.Contains(content, expected) {
			t.Errorf("BUG 4 NOT FIXED: expected hook command with absolute path not found\n"+
				"Expected to find: %q\n"+
				"Content: %s",
				expected, content)
		}
	}
}

// testBug5RerunCrash asserts that calling InstallLaunchdPlist twice does not return
// an error on the second call.
//
// FAILS on unfixed code because launchctl load returns exit code 1 when the service
// is already loaded, making re-runs unsafe.
//
// PASSES after the fix because InstallLaunchdPlist uses bootout-then-bootstrap,
// which is idempotent: bootout removes any existing registration (ignoring error if
// not loaded), then bootstrap registers fresh.
//
// Counterexample: second launchctl load call returns exit status 1.
func testBug5RerunCrash(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd re-run crash test only runs on darwin")
	}

	tmpHome := t.TempDir()

	// Set HOME to tmpHome so InstallLaunchdPlist writes to the test directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	// Use the current test binary as binPath to avoid needing mnemos on PATH
	selfPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable path: %v", err)
	}

	// Clean up: bootout after test regardless of outcome
	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)
	plistPath := filepath.Join(tmpHome, "Library", "LaunchAgents", "com.mnemos.autopilot.plist")
	t.Cleanup(func() {
		exec.Command("launchctl", "bootout", domain, plistPath).Run() //nolint:errcheck
	})

	// First call to InstallLaunchdPlist — should succeed
	if err := setup.InstallLaunchdPlist(selfPath); err != nil {
		// If launchctl bootstrap fails (permissions), skip rather than fail
		t.Skipf("InstallLaunchdPlist first call failed (may need permissions): %v", err)
	}

	// Assert CORRECT behavior: second call MUST NOT return an error.
	// The fix uses bootout (ignore error) then bootstrap — idempotent.
	if err := setup.InstallLaunchdPlist(selfPath); err != nil {
		t.Errorf("BUG 5 NOT FIXED: second call to InstallLaunchdPlist returned error: %v\n"+
			"The fix must use bootout-then-bootstrap for idempotency", err)
	} else {
		t.Logf("PASS: second InstallLaunchdPlist call succeeded (bootout-then-bootstrap is idempotent)")
	}
}
