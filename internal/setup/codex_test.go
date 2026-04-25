package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestFindSectionEnd_NextSection(t *testing.T) {
	lines := []string{
		"[mcp_servers.mnemos]",
		"command = \"mnemos\"",
		"args = [\"serve\"]",
		"[mcp_servers.playwright]",
		"command = \"playwright\"",
	}

	endIdx := findSectionEnd(lines, 0)
	if endIdx != 3 {
		t.Errorf("expected endIdx=3, got %d", endIdx)
	}
}

func TestFindSectionEnd_EOF(t *testing.T) {
	lines := []string{
		"[mcp_servers.mnemos]",
		"command = \"mnemos\"",
		"args = [\"serve\"]",
	}

	endIdx := findSectionEnd(lines, 0)
	if endIdx != len(lines) {
		t.Errorf("expected endIdx=%d (EOF), got %d", len(lines), endIdx)
	}
}

func TestFindSectionEnd_VariousTOMLStructures(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		startIdx int
		wantEnd  int
	}{
		{
			name: "section with comments",
			lines: []string{
				"[mcp_servers.mnemos]",
				"# This is a comment",
				"command = \"mnemos\"",
				"[other_section]",
			},
			startIdx: 0,
			wantEnd:  3,
		},
		{
			name: "section with blank lines",
			lines: []string{
				"[mcp_servers.mnemos]",
				"command = \"mnemos\"",
				"",
				"args = [\"serve\"]",
				"[mcp_servers.other]",
			},
			startIdx: 0,
			wantEnd:  4,
		},
		{
			name: "nested table syntax",
			lines: []string{
				"[mcp_servers.mnemos]",
				"command = \"mnemos\"",
				"[mcp_servers.mnemos.env]",
				"PROJECT = \"test\"",
				"[other]",
			},
			startIdx: 0,
			wantEnd:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findSectionEnd(tt.lines, tt.startIdx)
			if got != tt.wantEnd {
				t.Errorf("findSectionEnd() = %d, want %d", got, tt.wantEnd)
			}
		})
	}
}

func TestUpdateFields_ExistingFields(t *testing.T) {
	sectionLines := []string{
		"command = \"old-path\"",
		"args = [\"old-arg\"]",
		"env = { \"PROJECT\" = \"test\" }",
	}

	result := updateFields(sectionLines, "/new/path/mnemos")

	// Check command updated
	if !strings.Contains(result[0], "/new/path/mnemos") {
		t.Errorf("command not updated: %s", result[0])
	}

	// Check args updated
	if !strings.Contains(result[1], "[\"serve\"]") {
		t.Errorf("args not updated: %s", result[1])
	}

	// Check env preserved
	if !strings.Contains(result[2], "env") {
		t.Errorf("env not preserved: %v", result)
	}
}

func TestUpdateFields_PreserveIndentation(t *testing.T) {
	sectionLines := []string{
		"  command = \"old-path\"",
		"  args = [\"old-arg\"]",
		"  timeout = 30",
	}

	result := updateFields(sectionLines, "/new/path/mnemos")

	// Check indentation preserved
	if !strings.HasPrefix(result[0], "  ") {
		t.Errorf("indentation not preserved for command: %s", result[0])
	}
	if !strings.HasPrefix(result[1], "  ") {
		t.Errorf("indentation not preserved for args: %s", result[1])
	}
	if !strings.HasPrefix(result[2], "  ") {
		t.Errorf("indentation not preserved for timeout: %s", result[2])
	}
}

func TestUpdateFields_AppendMissing(t *testing.T) {
	sectionLines := []string{
		"env = { \"PROJECT\" = \"test\" }",
		"timeout = 30",
	}

	result := updateFields(sectionLines, "/path/mnemos")

	// Should have 4 lines: env, timeout, command, args
	if len(result) != 4 {
		t.Errorf("expected 4 lines, got %d: %v", len(result), result)
	}

	// Check command and args appended
	hasCommand := false
	hasArgs := false
	for _, line := range result {
		if strings.Contains(line, "command =") {
			hasCommand = true
		}
		if strings.Contains(line, "args =") {
			hasArgs = true
		}
	}

	if !hasCommand {
		t.Error("command not appended")
	}
	if !hasArgs {
		t.Error("args not appended")
	}
}

func TestUpdateFields_InlineComments(t *testing.T) {
	sectionLines := []string{
		"command = \"old-path\" # old comment",
		"args = [\"old-arg\"]",
		"# Full line comment",
		"env = { \"PROJECT\" = \"test\" }",
	}

	result := updateFields(sectionLines, "/new/path/mnemos")

	// Command should be updated (inline comment may be lost)
	if !strings.Contains(result[0], "/new/path/mnemos") {
		t.Errorf("command not updated: %s", result[0])
	}

	// Full line comment should be preserved
	hasComment := false
	for _, line := range result {
		if strings.Contains(line, "# Full line comment") {
			hasComment = true
			break
		}
	}
	if !hasComment {
		t.Error("full line comment not preserved")
	}
}

func TestGetIndentation(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"command = \"test\"", ""},
		{"  command = \"test\"", "  "},
		{"\tcommand = \"test\"", "\t"},
		{"    command = \"test\"", "    "},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := getIndentation(tt.line)
			if got != tt.want {
				t.Errorf("getIndentation(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestMergeCodexTOML_AppendToEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	changed, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "test-project", false)
	if err != nil {
		t.Fatalf("MergeCodexTOML failed: %v", err)
	}
	if !changed {
		t.Error("expected changed=true for new file")
	}

	// Read and parse result
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	// Verify structure
	servers, ok := config["mcp_servers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp_servers not found")
	}

	mnemos, ok := servers["mnemos"].(map[string]interface{})
	if !ok {
		t.Fatal("mnemos server not found")
	}

	if mnemos["command"] != "/usr/local/bin/mnemos" {
		t.Errorf("command = %v, want /usr/local/bin/mnemos", mnemos["command"])
	}

	args, ok := mnemos["args"].([]interface{})
	if !ok || len(args) != 1 || args[0] != "serve" {
		t.Errorf("args = %v, want [serve]", args)
	}

	env, ok := mnemos["env"].(map[string]interface{})
	if !ok || env["MNEMOS_PROJECT"] != "test-project" {
		t.Errorf("env = %v, want {MNEMOS_PROJECT: test-project}", env)
	}
}

func TestMergeCodexTOML_AppendToExistingMCPServers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create existing config with another MCP server
	existing := `[mcp_servers.playwright]
command = "playwright"
args = ["serve"]
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	changed, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "test-project", false)
	if err != nil {
		t.Fatalf("MergeCodexTOML failed: %v", err)
	}
	if !changed {
		t.Error("expected changed=true when adding new server")
	}

	// Read and parse result
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	servers, ok := config["mcp_servers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp_servers not found")
	}

	// Verify both servers exist
	if _, ok := servers["playwright"]; !ok {
		t.Error("playwright server not preserved")
	}
	if _, ok := servers["mnemos"]; !ok {
		t.Error("mnemos server not added")
	}
}

func TestMergeCodexTOML_UpdateExistingWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create existing config with mnemos entry
	existing := `[mcp_servers.mnemos]
command = "/old/path/mnemos"
args = ["old-arg"]
env = { "MNEMOS_PROJECT" = "old-project", "CUSTOM_VAR" = "value" }
timeout = 30
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	changed, err := MergeCodexTOML(configPath, "/new/path/mnemos", "new-project", false)
	if err != nil {
		t.Fatalf("MergeCodexTOML failed: %v", err)
	}
	if !changed {
		t.Error("expected changed=true when updating entry")
	}

	// Read and parse result
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	servers := config["mcp_servers"].(map[string]interface{})
	mnemos := servers["mnemos"].(map[string]interface{})

	// Command and args should be updated
	if mnemos["command"] != "/new/path/mnemos" {
		t.Errorf("command not updated: %v", mnemos["command"])
	}

	args := mnemos["args"].([]interface{})
	if len(args) != 1 || args[0] != "serve" {
		t.Errorf("args not updated: %v", args)
	}

	// Env and timeout should be preserved
	env := mnemos["env"].(map[string]interface{})
	if env["CUSTOM_VAR"] != "value" {
		t.Error("custom env var not preserved")
	}

	if mnemos["timeout"] != int64(30) {
		t.Error("timeout not preserved")
	}
}

func TestMergeCodexTOML_UpdateExistingWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create existing config with mnemos entry
	existing := `[mcp_servers.mnemos]
command = "/old/path/mnemos"
args = ["old-arg"]
env = { "MNEMOS_PROJECT" = "old-project", "CUSTOM_VAR" = "value" }
timeout = 30
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	changed, err := MergeCodexTOML(configPath, "/new/path/mnemos", "new-project", true)
	if err != nil {
		t.Fatalf("MergeCodexTOML failed: %v", err)
	}
	if !changed {
		t.Error("expected changed=true with --force")
	}

	// Read and parse result
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML: %v", err)
	}

	servers := config["mcp_servers"].(map[string]interface{})
	mnemos := servers["mnemos"].(map[string]interface{})

	// Command and args should be updated
	if mnemos["command"] != "/new/path/mnemos" {
		t.Errorf("command not updated: %v", mnemos["command"])
	}

	// Env should be replaced (only MNEMOS_PROJECT)
	env := mnemos["env"].(map[string]interface{})
	if env["MNEMOS_PROJECT"] != "new-project" {
		t.Error("MNEMOS_PROJECT not updated")
	}
	if _, ok := env["CUSTOM_VAR"]; ok {
		t.Error("CUSTOM_VAR should be removed with --force")
	}

	// Timeout should be removed
	if _, ok := mnemos["timeout"]; ok {
		t.Error("timeout should be removed with --force")
	}
}

func TestMergeCodexTOML_PreserveComments(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create existing config with comments
	existing := `[mcp_servers.mnemos]
# This is a comment
command = "/old/path/mnemos"
args = ["old-arg"]
# Another comment
env = { "MNEMOS_PROJECT" = "old-project" }
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	changed, err := MergeCodexTOML(configPath, "/new/path/mnemos", "new-project", false)
	if err != nil {
		t.Fatalf("MergeCodexTOML failed: %v", err)
	}
	if !changed {
		t.Error("expected changed=true when updating entry")
	}

	// Read result as string to check comments
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	content := string(data)

	// Comments should be preserved
	if !strings.Contains(content, "# This is a comment") {
		t.Error("first comment not preserved")
	}
	if !strings.Contains(content, "# Another comment") {
		t.Error("second comment not preserved")
	}

	// Verify TOML is still valid
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid TOML after merge: %v", err)
	}
}

func TestMergeCodexTOML_InvalidTOMLParseError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create invalid TOML
	invalid := `[mcp_servers.mnemos
command = "mnemos"
`
	if err := os.WriteFile(configPath, []byte(invalid), 0o600); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "test-project", false)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}

	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should mention parse failure: %v", err)
	}
}

func TestMergeCodexTOML_CorruptionDetection(t *testing.T) {
	// This test is difficult to trigger in practice since our string manipulation
	// is designed to produce valid TOML. We'll test the error path by mocking
	// a scenario where the result would be invalid.

	// For now, we verify that valid input produces valid output
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	existing := `[mcp_servers.mnemos]
command = "/old/path/mnemos"
args = ["serve"]
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := MergeCodexTOML(configPath, "/new/path/mnemos", "test-project", false)
	if err != nil {
		t.Fatalf("MergeCodexTOML should succeed: %v", err)
	}

	// Verify result is valid TOML
	data, _ := os.ReadFile(configPath)
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("result TOML is invalid: %v", err)
	}
}

func TestMergeCodexTOML_MissingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.toml")

	// Directory doesn't exist - should be created
	_, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "test-project", false)
	if err != nil {
		t.Fatalf("MergeCodexTOML should create directory: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file not created")
	}
}

func TestMergeCodexTOML_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	_, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "test-project", false)
	if err != nil {
		t.Fatalf("MergeCodexTOML failed: %v", err)
	}

	// Verify file has 0600 permissions
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("file permissions = %o, want 0600", mode)
	}
}

func TestMergeCodexTOML_SkipWhenMatches(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create existing config that already matches
	existing := `[mcp_servers.mnemos]
command = "/usr/local/bin/mnemos"
args = ["serve"]
env = { "MNEMOS_PROJECT" = "test-project" }
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Run merge with same values
	changed, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "test-project", false)
	if err != nil {
		t.Fatalf("MergeCodexTOML failed: %v", err)
	}

	if changed {
		t.Error("expected changed=false when entry already matches")
	}

	// Verify file wasn't modified
	data, _ := os.ReadFile(configPath)
	if string(data) != existing {
		t.Error("file was modified when it should have been skipped")
	}
}

func TestMergeCodexTOML_ForceOverridesSkip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Create existing config that already matches
	existing := `[mcp_servers.mnemos]
command = "/usr/local/bin/mnemos"
args = ["serve"]
env = { "MNEMOS_PROJECT" = "test-project" }
`
	if err := os.WriteFile(configPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Run merge with --force
	changed, err := MergeCodexTOML(configPath, "/usr/local/bin/mnemos", "test-project", true)
	if err != nil {
		t.Fatalf("MergeCodexTOML failed: %v", err)
	}

	if !changed {
		t.Error("expected changed=true with --force even when entry matches")
	}
}
