package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MnemosMCPEntry is the standard entry for Mnemos MCP server
type MnemosMCPEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// MergeClaudeSettings reads the existing .claude/settings.json (if any), merges
// the mnemos hook entries using binPath as the absolute binary path, and writes it
// back. Idempotent — re-running with a different binPath rewrites existing mnemos
// hook entries in-place rather than creating duplicates.
func MergeClaudeSettings(filePath, binPath string) error {
	type hookEntry struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type hookGroup struct {
		Hooks []hookEntry `json:"hooks"`
	}

	var root map[string]json.RawMessage

	data, err := os.ReadFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return err
		}
	}
	if root == nil {
		root = make(map[string]json.RawMessage)
	}

	// Get or create the hooks map
	var hooks map[string]json.RawMessage
	if raw, ok := root["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return err
		}
	}
	if hooks == nil {
		hooks = make(map[string]json.RawMessage)
	}

	// Mnemos hook event → command suffix mapping.
	// The full command is: binPath + " hook " + suffix
	mnemosEvents := map[string]string{
		"SessionStart":     "session-start",
		"UserPromptSubmit": "prompt-submit",
		"SessionEnd":       "session-end",
	}

	for event, suffix := range mnemosEvents {
		desiredCmd := binPath + " hook " + suffix

		// Decode existing groups for this event (may be nil)
		var groups []hookGroup
		if raw, ok := hooks[event]; ok {
			if err := json.Unmarshal(raw, &groups); err != nil {
				return err
			}
		}

		// Look for an existing mnemos hook entry (any path prefix) and rewrite it.
		// We match on the hook suffix ("hook session-start" etc.) so that re-running
		// setup with a different binPath updates the path in-place rather than
		// creating a duplicate entry.
		found := false
		for gi := range groups {
			for hi := range groups[gi].Hooks {
				h := &groups[gi].Hooks[hi]
				if strings.Contains(h.Command, "hook "+suffix) {
					h.Command = desiredCmd // rewrite with new binPath
					found = true
				}
			}
		}

		if !found {
			groups = append(groups, hookGroup{
				Hooks: []hookEntry{{Type: "command", Command: desiredCmd}},
			})
		}

		encoded, err := json.Marshal(groups)
		if err != nil {
			return err
		}
		hooks[event] = json.RawMessage(encoded)
	}

	// Re-encode hooks back into root
	hooksBytes, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	root["hooks"] = json.RawMessage(hooksBytes)

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	// Atomic write
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".claude-settings-merge-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filePath); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// and writes it back. Idempotent — calling multiple times does not create duplicates.
func MergeMCPConfig(filePath string, serverName string, entry MnemosMCPEntry) error {
	// Top-level map: keys are "mcpServers" etc.
	var root map[string]json.RawMessage

	data, err := os.ReadFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return err
		}
	}

	if root == nil {
		root = make(map[string]json.RawMessage)
	}

	// Get or create the mcpServers map
	var servers map[string]json.RawMessage
	if raw, ok := root["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return err
		}
	}
	if servers == nil {
		servers = make(map[string]json.RawMessage)
	}

	// Read existing entry (if any) as raw map to preserve extra fields (env, timeout, etc.)
	var existingEntry map[string]json.RawMessage
	if raw, ok := servers[serverName]; ok {
		json.Unmarshal(raw, &existingEntry) //nolint:errcheck — best-effort; nil on failure is fine
	}
	if existingEntry == nil {
		existingEntry = make(map[string]json.RawMessage)
	}
	// Update only command and args — preserve env, timeout, and any other keys
	existingEntry["command"], _ = json.Marshal(entry.Command)
	existingEntry["args"], _ = json.Marshal(entry.Args)
	entryBytes, err := json.Marshal(existingEntry)
	if err != nil {
		return err
	}
	servers[serverName] = json.RawMessage(entryBytes)

	// Re-encode mcpServers back into root
	serversBytes, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	root["mcpServers"] = json.RawMessage(serversBytes)

	// Marshal the full document with indentation
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	// Atomic write: temp file + rename
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".mcp-merge-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, filePath); err != nil {
		os.Remove(tmpName)
		return err
	}

	return nil
}

// MergeClaudeGlobalMCP registers the mnemos MCP server in the Claude global config.
// Strategy 1: use the claude CLI (idempotent: remove then add).
// Strategy 2 (fallback): merge directly into ~/.claude.json.
func MergeClaudeGlobalMCP(binPath string) error {
	claudePath, err := exec.LookPath("claude")
	if err == nil {
		// Remove first (ignore error — may not exist)
		exec.Command(claudePath, "mcp", "remove", "--scope", "user", "mnemos").Run() //nolint:errcheck

		// Add with absolute binPath
		addCmd := exec.Command(claudePath, "mcp", "add", "--scope", "user", "mnemos", "--", binPath, "serve")
		if err := addCmd.Run(); err == nil {
			return nil
		}
		// fall through to strategy 2 on non-zero exit
	}

	// Strategy 2: direct file merge
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	claudeJSON := filepath.Join(home, ".claude.json")
	return MergeMCPConfig(claudeJSON, "mnemos", MnemosMCPEntry{
		Command: binPath,
		Args:    []string{"serve"},
	})
}

// RemoveMCPEntry removes a server entry from the mcpServers map in a JSON file.
// Atomic write. No-op if the file or key does not exist.
func RemoveMCPEntry(filePath, serverName string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no-op
		}
		return err
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}

	rawServers, ok := root["mcpServers"]
	if !ok {
		return nil // no mcpServers key — no-op
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(rawServers, &servers); err != nil {
		return err
	}

	if _, ok := servers[serverName]; !ok {
		return nil // key not present — no-op
	}

	delete(servers, serverName)

	serversBytes, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	root["mcpServers"] = json.RawMessage(serversBytes)

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')

	// Atomic write
	dir := filepath.Dir(filePath)
	tmp, err := os.CreateTemp(dir, ".mcp-remove-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, filePath)
}
