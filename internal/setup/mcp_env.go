package setup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// MergeMCPEnv merges env vars into an existing MCP server entry under mcpServers[serverName].
// It preserves all existing fields and env keys. Missing parent structures are created.
// Idempotent.
func MergeMCPEnv(filePath string, serverName string, env map[string]string) error {
	if len(env) == 0 {
		return nil
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

	var servers map[string]json.RawMessage
	if raw, ok := root["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return err
		}
	}
	if servers == nil {
		servers = make(map[string]json.RawMessage)
	}

	var entry map[string]json.RawMessage
	if raw, ok := servers[serverName]; ok {
		_ = json.Unmarshal(raw, &entry)
	}
	if entry == nil {
		entry = make(map[string]json.RawMessage)
	}

	var existingEnv map[string]string
	if raw, ok := entry["env"]; ok {
		_ = json.Unmarshal(raw, &existingEnv)
	}
	if existingEnv == nil {
		existingEnv = make(map[string]string)
	}
	for k, v := range env {
		existingEnv[k] = v
	}
	entry["env"], _ = json.Marshal(existingEnv)

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	servers[serverName] = json.RawMessage(entryBytes)

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

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".mcp-env-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filePath); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	return nil
}

