package setup

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RemoveHooksByPrefix reads the Claude settings.json at filePath, removes all
// hook entries whose command field has the given prefix, and writes the result
// back atomically. Full 3-level cleanup prevents leaving empty artifacts:
//  1. Remove entries from inner hooks[] where command has prefix
//  2. Remove groups where inner hooks[] is now empty
//  3. Remove event key where event array is now empty
//  4. Remove "hooks" key from root if hooks object is now empty
//
// No-op if the file does not exist.
func RemoveHooksByPrefix(filePath, cmdPrefix string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no-op
		}
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse %s: %w", filePath, err)
	}

	rawHooks, ok := root["hooks"]
	if !ok {
		return nil // no hooks key — no-op
	}

	// Decode the hooks object: map[eventName][]group
	var hooksMap map[string]json.RawMessage
	if err := json.Unmarshal(rawHooks, &hooksMap); err != nil {
		return fmt.Errorf("parse hooks in %s: %w", filePath, err)
	}

	type hookEntry struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type hookGroup struct {
		Hooks []hookEntry `json:"hooks"`
	}

	// Process each event key
	for event, rawGroups := range hooksMap {
		var groups []hookGroup
		if err := json.Unmarshal(rawGroups, &groups); err != nil {
			// Skip unparseable event entries
			continue
		}

		// Step 1: Remove matching entries from each group's inner hooks[]
		var filteredGroups []hookGroup
		for _, g := range groups {
			var filteredHooks []hookEntry
			for _, h := range g.Hooks {
				if !strings.HasPrefix(h.Command, cmdPrefix) {
					filteredHooks = append(filteredHooks, h)
				}
			}
			// Step 2: Remove groups where inner hooks[] is now empty
			if len(filteredHooks) > 0 {
				g.Hooks = filteredHooks
				filteredGroups = append(filteredGroups, g)
			}
		}

		// Step 3: Remove event key where event array is now empty
		if len(filteredGroups) == 0 {
			delete(hooksMap, event)
		} else {
			encoded, err := json.Marshal(filteredGroups)
			if err != nil {
				return fmt.Errorf("encode groups for event %s: %w", event, err)
			}
			hooksMap[event] = json.RawMessage(encoded)
		}
	}

	// Step 4: Remove "hooks" key from root if hooks object is now empty
	if len(hooksMap) == 0 {
		delete(root, "hooks")
	} else {
		hooksBytes, err := json.Marshal(hooksMap)
		if err != nil {
			return fmt.Errorf("encode hooks: %w", err)
		}
		root["hooks"] = json.RawMessage(hooksBytes)
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filePath, err)
	}
	out = append(out, '\n')

	// Atomic write
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".hooks-remove-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, filePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// RemoveSentinelBlock removes the content between beginMarker and endMarker
// (inclusive) from the file at filePath. Warns and skips if either marker is
// absent. Atomic write.
func RemoveSentinelBlock(filePath, beginMarker, endMarker string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no-op
		}
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	content := string(data)

	beginIdx := strings.Index(content, beginMarker)
	endIdx := strings.Index(content, endMarker)

	if beginIdx == -1 || endIdx == -1 {
		log.Printf("Warning: sentinel markers not found in %s — skipping removal", filePath)
		return nil
	}

	// endIdx points to the start of endMarker; advance past it
	endIdx += len(endMarker)

	// Remove the block (including any trailing newline after the end marker)
	before := content[:beginIdx]
	after := content[endIdx:]

	// Trim a single leading newline from `after` to avoid leaving a blank line
	after = strings.TrimPrefix(after, "\n")

	result := before + after

	// Atomic write
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".sentinel-remove-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(result); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, filePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// UninstallClaudeGlobal removes all global mnemos setup artifacts in order:
//  1. UninstallLaunchdPlist (darwin-only via build tag; no-op on other platforms)
//  2. Try `claude mcp remove --scope user mnemos`; on failure, RemoveMCPEntry("~/.claude.json", "mnemos")
//  3. RemoveHooksByPrefix("~/.claude/settings.json", "mnemos hook")
//  4. RemoveSentinelBlock("~/CLAUDE.md", "<!-- mnemos:begin -->", "<!-- mnemos:end -->")
func UninstallClaudeGlobal(binPath string) error {
	// Step 1: Remove launchd plist (darwin-only; no-op on other platforms)
	if err := UninstallLaunchdPlist(); err != nil {
		return fmt.Errorf("uninstall launchd plist: %w", err)
	}

	// Step 2: Remove global MCP entry
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}

	claudePath, lookErr := exec.LookPath("claude")
	if lookErr == nil {
		cmd := exec.Command(claudePath, "mcp", "remove", "--scope", "user", "mnemos")
		if err := cmd.Run(); err != nil {
			// Fall back to direct file removal
			claudeJSON := filepath.Join(home, ".claude.json")
			if err := RemoveMCPEntry(claudeJSON, "mnemos"); err != nil {
				return fmt.Errorf("remove MCP entry from ~/.claude.json: %w", err)
			}
		}
	} else {
		// claude CLI not found — remove directly from file
		claudeJSON := filepath.Join(home, ".claude.json")
		if err := RemoveMCPEntry(claudeJSON, "mnemos"); err != nil {
			return fmt.Errorf("remove MCP entry from ~/.claude.json: %w", err)
		}
	}

	// Step 3: Remove hooks from ~/.claude/settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := RemoveHooksByPrefix(settingsPath, "mnemos hook"); err != nil {
		return fmt.Errorf("remove hooks from ~/.claude/settings.json: %w", err)
	}

	// Step 4: Remove sentinel block from ~/CLAUDE.md
	claudeMD := filepath.Join(home, "CLAUDE.md")
	if err := RemoveSentinelBlock(claudeMD, "<!-- mnemos:begin -->", "<!-- mnemos:end -->"); err != nil {
		return fmt.Errorf("remove sentinel block from ~/CLAUDE.md: %w", err)
	}

	return nil
}

// UninstallClaudeLocal removes local mnemos setup artifacts in order:
//  1. RemoveMCPEntry(".mcp.json", "mnemos")
//  2. RemoveHooksByPrefix(".claude/settings.json", "mnemos hook")
//  3. RemoveSentinelBlock("./CLAUDE.md", "<!-- mnemos:begin -->", "<!-- mnemos:end -->")
func UninstallClaudeLocal() error {
	// Step 1: Remove local MCP entry
	if err := RemoveMCPEntry(".mcp.json", "mnemos"); err != nil {
		return fmt.Errorf("remove MCP entry from .mcp.json: %w", err)
	}

	// Step 2: Remove hooks from .claude/settings.json
	if err := RemoveHooksByPrefix(".claude/settings.json", "mnemos hook"); err != nil {
		return fmt.Errorf("remove hooks from .claude/settings.json: %w", err)
	}

	// Step 3: Remove sentinel block from ./CLAUDE.md
	if err := RemoveSentinelBlock("./CLAUDE.md", "<!-- mnemos:begin -->", "<!-- mnemos:end -->"); err != nil {
		return fmt.Errorf("remove sentinel block from ./CLAUDE.md: %w", err)
	}

	return nil
}
