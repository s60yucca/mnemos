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

// MergeMCPConfig reads the existing JSON file (if any), merges the Mnemos entry,
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
