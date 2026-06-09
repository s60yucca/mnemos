package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// TestCodexIntegration_FreshInstall tests a complete fresh install scenario.
func TestCodexIntegration_FreshInstall(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".codex", "config.toml")

	changed, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "my-project", false)
	if err != nil {
		t.Fatalf("fresh install failed: %v", err)
	}
	if !changed {
		t.Error("expected changed=true for fresh install")
	}

	// Verify file exists
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Verify valid TOML
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	// Verify structure
	servers := config["mcp_servers"].(map[string]interface{})
	mnemos := servers["mnemos"].(map[string]interface{})

	if mnemos["command"] != "/usr/local/bin/mnemos" {
		t.Errorf("command = %v", mnemos["command"])
	}
	args := mnemos["args"].([]interface{})
	if len(args) != 1 || args[0] != "serve" {
		t.Errorf("args = %v", args)
	}
	env := mnemos["env"].(map[string]interface{})
	if env["MNEMOS_PROJECT_ID"] != "my-project" {
		t.Errorf("MNEMOS_PROJECT_ID = %v", env["MNEMOS_PROJECT_ID"])
	}
}

// TestCodexIntegration_UpdateExistingConfig tests updating a config that has other servers.
func TestCodexIntegration_UpdateExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Existing config with another server
	existing := `[mcp_servers.playwright]
command = "playwright-mcp"
args = ["--port", "3000"]

[mcp_servers.github]
command = "github-mcp"
args = ["serve"]
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "my-project", false)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML after update: %v", err)
	}

	servers := config["mcp_servers"].(map[string]interface{})

	// All three servers should exist
	if _, ok := servers["playwright"]; !ok {
		t.Error("playwright server lost")
	}
	if _, ok := servers["github"]; !ok {
		t.Error("github server lost")
	}
	if _, ok := servers["mnemos"]; !ok {
		t.Error("mnemos server not added")
	}

	// Verify other servers unchanged
	playwright := servers["playwright"].(map[string]interface{})
	if playwright["command"] != "playwright-mcp" {
		t.Error("playwright command changed")
	}
}

// TestCodexIntegration_ForceUpdate tests --force replacing the entire entry.
func TestCodexIntegration_ForceUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	existing := `[mcp_servers.mnemos]
command = "/old/path/mnemos"
args = ["serve"]
env = { "MNEMOS_PROJECT_ID" = "old-project", "CUSTOM_VAR" = "keep-me" }
timeout = 60
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, err := MergeCodexTOML(configPath, "/new/path/mnemos", "new-project", true)
	if err != nil {
		t.Fatalf("force update failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	servers := config["mcp_servers"].(map[string]interface{})
	mnemos := servers["mnemos"].(map[string]interface{})

	if mnemos["command"] != "/new/path/mnemos" {
		t.Errorf("command not updated: %v", mnemos["command"])
	}

	env := mnemos["env"].(map[string]interface{})
	if env["MNEMOS_PROJECT_ID"] != "new-project" {
		t.Error("MNEMOS_PROJECT_ID not updated")
	}
	if _, ok := env["CUSTOM_VAR"]; ok {
		t.Error("CUSTOM_VAR should be removed with --force")
	}
	if _, ok := mnemos["timeout"]; ok {
		t.Error("timeout should be removed with --force")
	}
}

// TestCodexIntegration_CustomProjectID tests --project flag.
func TestCodexIntegration_CustomProjectID(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	_, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "custom-project-id", false)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)

	if !strings.Contains(content, "custom-project-id") {
		t.Error("custom project ID not in config")
	}
}

// TestCodexIntegration_Idempotency tests that running setup twice produces the same result.
func TestCodexIntegration_Idempotency(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// First run
	changed1, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "my-project", false)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if !changed1 {
		t.Error("expected changed=true on first run")
	}

	data1, _ := os.ReadFile(configPath)

	// Second run — same args
	changed2, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "my-project", false)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if changed2 {
		t.Error("expected changed=false on second run (idempotent)")
	}

	data2, _ := os.ReadFile(configPath)

	// File should be identical
	if string(data1) != string(data2) {
		t.Errorf("file changed on second run:\nfirst:\n%s\nsecond:\n%s", data1, data2)
	}

	// Verify no duplicate entries
	var config map[string]interface{}
	if err := toml.Unmarshal(data2, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	servers := config["mcp_servers"].(map[string]interface{})
	if len(servers) != 1 {
		t.Errorf("expected 1 server, got %d (duplicate?)", len(servers))
	}
}

// TestCodexIntegration_IdempotencyWithExistingServers tests idempotency with other servers present.
func TestCodexIntegration_IdempotencyWithExistingServers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	existing := `[mcp_servers.playwright]
command = "playwright-mcp"
args = ["serve"]
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Run twice
	for i := 0; i < 2; i++ {
		_, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "my-project", false)
		if err != nil {
			t.Fatalf("run %d failed: %v", i+1, err)
		}
	}

	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	servers := config["mcp_servers"].(map[string]interface{})
	if len(servers) != 2 {
		t.Errorf("expected 2 servers (playwright + mnemos), got %d", len(servers))
	}
}

// TestCodexIntegration_PreserveOtherServers verifies other MCP servers are untouched.
func TestCodexIntegration_PreserveOtherServers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	existing := `[mcp_servers.playwright]
command = "playwright-mcp"
args = ["--port", "3000"]
env = { "PLAYWRIGHT_TIMEOUT" = "30000" }

[mcp_servers.github]
command = "github-mcp"
args = ["serve", "--verbose"]
timeout = 120
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "my-project", false)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	servers := config["mcp_servers"].(map[string]interface{})

	// Playwright preserved
	playwright := servers["playwright"].(map[string]interface{})
	if playwright["command"] != "playwright-mcp" {
		t.Error("playwright command changed")
	}
	playwrightArgs := playwright["args"].([]interface{})
	if len(playwrightArgs) != 2 {
		t.Errorf("playwright args changed: %v", playwrightArgs)
	}
	playwrightEnv := playwright["env"].(map[string]interface{})
	if playwrightEnv["PLAYWRIGHT_TIMEOUT"] != "30000" {
		t.Error("playwright env changed")
	}

	// GitHub preserved
	github := servers["github"].(map[string]interface{})
	if github["command"] != "github-mcp" {
		t.Error("github command changed")
	}
	if github["timeout"] != int64(120) {
		t.Error("github timeout changed")
	}
}

// TestCodexIntegration_PreserveCustomEnvVars verifies custom env vars preserved without --force.
func TestCodexIntegration_PreserveCustomEnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	existing := `[mcp_servers.mnemos]
command = "/old/path/mnemos"
args = ["serve"]
env = { "MNEMOS_PROJECT_ID" = "old-project", "CUSTOM_DEBUG" = "true", "EXTRA_VAR" = "value" }
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, err := MergeCodexTOML(configPath, "/new/path/mnemos", "new-project", false)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	servers := config["mcp_servers"].(map[string]interface{})
	mnemos := servers["mnemos"].(map[string]interface{})

	// Command updated
	if mnemos["command"] != "/new/path/mnemos" {
		t.Error("command not updated")
	}

	// Custom env vars preserved
	env := mnemos["env"].(map[string]interface{})
	if env["CUSTOM_DEBUG"] != "true" {
		t.Error("CUSTOM_DEBUG not preserved")
	}
	if env["EXTRA_VAR"] != "value" {
		t.Error("EXTRA_VAR not preserved")
	}
}

// TestCodexIntegration_PreserveCommentsInSections verifies comments within sections are kept.
func TestCodexIntegration_PreserveCommentsInSections(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	existing := `# Global config comment
[mcp_servers.mnemos]
# mnemos memory server
command = "/old/path/mnemos"
args = ["serve"]
# project scoping
env = { "MNEMOS_PROJECT_ID" = "old-project" }
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, err := MergeCodexTOML(configPath, "/new/path/mnemos", "new-project", false)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)

	// Comments within section should be preserved
	if !strings.Contains(content, "# mnemos memory server") {
		t.Error("section comment not preserved")
	}
	if !strings.Contains(content, "# project scoping") {
		t.Error("inline comment not preserved")
	}

	// TOML still valid
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}
}
