package setup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// MnemosMCPEntry is the standard entry for Mnemos MCP server
type MnemosMCPEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// MergeClaudeSettings reads the existing .claude/settings.json (if any), merges
// the mnemos hook entries, and writes it back. Idempotent — calling multiple times
// does not create duplicate hook entries.
func MergeClaudeSettings(filePath string) error {
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

	// Mnemos hook commands to inject
	mnemosCmds := map[string]string{
		"SessionStart":     "mnemos hook session-start",
		"UserPromptSubmit": "mnemos hook prompt-submit",
		"SessionEnd":       "mnemos hook session-end",
	}

	for event, cmd := range mnemosCmds {
		// Decode existing groups for this event (may be nil)
		var groups []hookGroup
		if raw, ok := hooks[event]; ok {
			if err := json.Unmarshal(raw, &groups); err != nil {
				return err
			}
		}

		// Check if mnemos command already present in any group
		alreadyPresent := false
		for _, g := range groups {
			for _, h := range g.Hooks {
				if h.Command == cmd {
					alreadyPresent = true
					break
				}
			}
			if alreadyPresent {
				break
			}
		}

		if !alreadyPresent {
			groups = append(groups, hookGroup{
				Hooks: []hookEntry{{Type: "command", Command: cmd}},
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

	// Encode the entry and set it (overwrites if exists — idempotent)
	entryBytes, err := json.Marshal(entry)
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
