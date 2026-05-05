package setup

import (
	"fmt"
	"os"
	"path/filepath"
)

func TraeGlobalMCPPaths() ([]string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config dir: %w", err)
	}

	candidates := []string{
		filepath.Join(cfgDir, "TRAE SOLO", "User", "mcp.json"),
		filepath.Join(cfgDir, "Trae", "User", "mcp.json"),
	}

	var paths []string
	for _, p := range candidates {
		parent := filepath.Dir(p)
		if _, err := os.Stat(parent); err == nil {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// MergeTraeGlobalMCP registers mnemos in Trae / Trae Solo global MCP config, when present.
// Returns the list of config paths that were updated.
func MergeTraeGlobalMCP(binPath string) ([]string, error) {
	paths, err := TraeGlobalMCPPaths()
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	for _, p := range paths {
		if err := MergeMCPConfig(p, "mnemos", MnemosMCPEntry{
			Command: binPath,
			Args:    []string{"serve"},
		}); err != nil {
			return nil, err
		}
		if err := MergeMCPEnv(p, "mnemos", map[string]string{
			"MNEMOS_CLIENT": "trae-solo",
		}); err != nil {
			return nil, err
		}
	}

	return paths, nil
}

