package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// MergeCodexTOML merges the mnemos MCP server entry into Codex's TOML configuration.
// It uses string-level manipulation with TOML parsing for validation.
//
// Parameters:
//   - filePath: Path to the config.toml file (typically ~/.codex/config.toml)
//   - binPath: Absolute path to the mnemos binary
//   - projectID: Project identifier for MNEMOS_PROJECT env var
//   - force: If true, replaces entire mnemos entry; if false, updates only command/args
//
// Returns (changed bool, error). changed is true if the file was modified.
// Returns (false, nil) if entry exists and matches (unless force=true).
func MergeCodexTOML(filePath string, binPath string, projectID string, force bool) (bool, error) {
	// Read existing file or start with empty content
	data, err := os.ReadFile(filePath)
	originalValid := false
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", filePath, err)
	}

	// Parse original TOML to validate syntax
	if len(data) > 0 {
		var testConfig map[string]interface{}
		if err := toml.Unmarshal(data, &testConfig); err != nil {
			return false, fmt.Errorf("parse %s: %w\nPlease fix TOML syntax errors before running setup", filePath, err)
		}
		originalValid = true
	}

	content := string(data)

	// Check if [mcp_servers.mnemos] exists
	mnemosExists := strings.Contains(content, "[mcp_servers.mnemos]")

	// If entry exists and not forcing, check if it already matches
	if mnemosExists && !force {
		if entryMatches(content, binPath) {
			// No changes needed
			return false, nil
		}
	}

	var result string
	if !mnemosExists {
		// Case 1: Append new entry
		result = appendMnemosEntry(content, binPath, projectID)
	} else {
		// Case 2: Update existing entry
		result = updateMnemosEntry(content, binPath, projectID, force)
	}

	// Validate result TOML
	var testConfig map[string]interface{}
	if err := toml.Unmarshal([]byte(result), &testConfig); err != nil {
		if originalValid {
			return false, fmt.Errorf("string manipulation produced invalid TOML (original was valid)\n"+
				"This is likely a bug in the setup command.\n"+
				"Workaround: Use --force flag to replace the entire entry.\n"+
				"Please report this issue with your config file at: https://github.com/mnemos-dev/mnemos/issues\n"+
				"Parse error: %w", err)
		}
		return false, fmt.Errorf("result TOML is invalid: %w", err)
	}

	// Atomic write
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("create config directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".codex-config-*.tmp")
	if err != nil {
		return false, fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	success := false
	defer func() {
		if !success {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.WriteString(result); err != nil {
		tmp.Close()
		return false, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpName, 0o600); err != nil {
		return false, fmt.Errorf("set file permissions: %w", err)
	}

	if err := os.Rename(tmpName, filePath); err != nil {
		return false, fmt.Errorf("rename temp file: %w", err)
	}

	success = true
	return true, nil
}

// appendMnemosEntry appends a new [mcp_servers.mnemos] entry to the TOML content.
func appendMnemosEntry(content string, binPath string, projectID string) string {
	// Check if [mcp_servers] section exists (any variant)
	hasMCPServers := strings.Contains(content, "[mcp_servers")

	var result strings.Builder
	result.WriteString(strings.TrimRight(content, "\n"))

	if !hasMCPServers {
		// No mcp_servers section at all - create it
		if len(content) > 0 {
			result.WriteString("\n\n")
		}
	} else {
		// mcp_servers exists - find end of last mcp_servers.X section
		result.WriteString("\n\n")
	}

	// Append new mnemos entry
	result.WriteString("[mcp_servers.mnemos]\n")
	result.WriteString(fmt.Sprintf("command = %q\n", binPath))
	result.WriteString("args = [\"serve\"]\n")
	result.WriteString(fmt.Sprintf("env = { \"MNEMOS_PROJECT\" = %q }\n", projectID))

	return result.String()
}

// updateMnemosEntry updates an existing [mcp_servers.mnemos] entry.
func updateMnemosEntry(content string, binPath string, projectID string, force bool) string {
	lines := strings.Split(content, "\n")

	// Find [mcp_servers.mnemos] section
	startIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[mcp_servers.mnemos]" {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		// Section not found (shouldn't happen, but handle gracefully)
		return appendMnemosEntry(content, binPath, projectID)
	}

	// Find section end
	endIdx := findSectionEnd(lines, startIdx)

	if force {
		// Full replacement
		var result strings.Builder
		// Keep everything before the section
		for i := 0; i < startIdx; i++ {
			result.WriteString(lines[i])
			result.WriteString("\n")
		}
		// Write new section
		result.WriteString("[mcp_servers.mnemos]\n")
		result.WriteString(fmt.Sprintf("command = %q\n", binPath))
		result.WriteString("args = [\"serve\"]\n")
		result.WriteString(fmt.Sprintf("env = { \"MNEMOS_PROJECT\" = %q }\n", projectID))
		// Keep everything after the section
		for i := endIdx; i < len(lines); i++ {
			result.WriteString(lines[i])
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
		}
		return result.String()
	}

	// Partial update: preserve env and other fields
	sectionLines := lines[startIdx+1 : endIdx]
	updatedSection := updateFields(sectionLines, binPath)

	var result strings.Builder
	// Keep everything before the section
	for i := 0; i < startIdx; i++ {
		result.WriteString(lines[i])
		result.WriteString("\n")
	}
	// Write section header
	result.WriteString("[mcp_servers.mnemos]\n")
	// Write updated section
	for _, line := range updatedSection {
		result.WriteString(line)
		result.WriteString("\n")
	}
	// Keep everything after the section
	for i := endIdx; i < len(lines); i++ {
		result.WriteString(lines[i])
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// findSectionEnd finds the end of a TOML section.
// Returns the index of the next section header or EOF.
func findSectionEnd(lines []string, startIdx int) int {
	for i := startIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") {
			return i // Next section starts
		}
	}
	return len(lines) // EOF
}

// updateFields updates command and args fields in a section, preserving other fields.
func updateFields(sectionLines []string, binPath string) []string {
	commandPattern := regexp.MustCompile(`^\s*command\s*=`)
	argsPattern := regexp.MustCompile(`^\s*args\s*=`)

	commandFound := false
	argsFound := false
	result := make([]string, 0, len(sectionLines))

	for _, line := range sectionLines {
		if commandPattern.MatchString(line) {
			// Update command field, preserving indentation
			indent := getIndentation(line)
			result = append(result, fmt.Sprintf("%scommand = %q", indent, binPath))
			commandFound = true
		} else if argsPattern.MatchString(line) {
			// Update args field, preserving indentation
			indent := getIndentation(line)
			result = append(result, fmt.Sprintf("%sargs = [\"serve\"]", indent))
			argsFound = true
		} else {
			// Keep other fields unchanged
			result = append(result, line)
		}
	}

	// Append missing fields
	if !commandFound {
		result = append(result, fmt.Sprintf("command = %q", binPath))
	}
	if !argsFound {
		result = append(result, "args = [\"serve\"]")
	}

	return result
}

// getIndentation extracts the leading whitespace from a line.
func getIndentation(line string) string {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return line[:i]
		}
	}
	return ""
}

// entryMatches checks if the existing mnemos entry already has the correct command and args.
func entryMatches(content string, binPath string) bool {
	lines := strings.Split(content, "\n")

	// Find [mcp_servers.mnemos] section
	startIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[mcp_servers.mnemos]" {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return false
	}

	// Find section end
	endIdx := findSectionEnd(lines, startIdx)
	sectionLines := lines[startIdx+1 : endIdx]

	// Check if command and args match
	hasCorrectCommand := false
	hasCorrectArgs := false

	commandPattern := regexp.MustCompile(`^\s*command\s*=\s*"` + regexp.QuoteMeta(binPath) + `"`)
	argsPattern := regexp.MustCompile(`^\s*args\s*=\s*\["serve"\]`)

	for _, line := range sectionLines {
		if commandPattern.MatchString(line) {
			hasCorrectCommand = true
		}
		if argsPattern.MatchString(line) {
			hasCorrectArgs = true
		}
	}

	return hasCorrectCommand && hasCorrectArgs
}
