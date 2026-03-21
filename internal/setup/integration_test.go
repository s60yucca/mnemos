package setup_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_SetupThenHooks verifies that after running setup for each client,
// all expected files exist and the MCP config contains the mnemos entry.
func TestIntegration_SetupThenHooks(t *testing.T) {
	clients := []string{"claude", "kiro", "cursor"}

	for _, clientName := range clients {
		t.Run(clientName, func(t *testing.T) {
			projectDir := t.TempDir()
			writer := setup.NewWriter(projectDir, false, true) // force=true to skip prompts

			clientCfg, ok := setup.Clients[clientName]
			require.True(t, ok, "client %q should be registered", clientName)

			// Write all template files
			for _, fm := range clientCfg.Files {
				content, err := setup.GetTemplate(fm.TemplatePath)
				require.NoError(t, err, "template %s should be readable", fm.TemplatePath)

				targetPath := filepath.Join(projectDir, fm.LocalPath)
				require.NoError(t, writer.EnsureDir(filepath.Dir(targetPath)))

				written, err := writer.WriteFile(targetPath, content)
				require.NoError(t, err)
				assert.True(t, written, "file should be written: %s", targetPath)

				// Verify file exists and is non-empty
				info, err := os.Stat(targetPath)
				require.NoError(t, err, "file should exist: %s", targetPath)
				assert.Greater(t, info.Size(), int64(0), "file should be non-empty: %s", targetPath)
			}

			// Merge MCP config
			mcpPath := filepath.Join(projectDir, clientCfg.MCPConfig.LocalPath)
			err := setup.MergeMCPConfig(mcpPath, "mnemos", setup.MnemosMCPEntry{
				Command: "mnemos",
				Args:    []string{"serve"},
			})
			require.NoError(t, err)

			// Verify MCP config has the mnemos entry
			data, err := os.ReadFile(mcpPath)
			require.NoError(t, err)

			var mcpDoc map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &mcpDoc))

			var servers map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(mcpDoc["mcpServers"], &servers))
			assert.Contains(t, servers, "mnemos", "mcpServers should contain mnemos entry")

			// Verify idempotency: merge again, still only one entry
			require.NoError(t, setup.MergeMCPConfig(mcpPath, "mnemos", setup.MnemosMCPEntry{
				Command: "mnemos",
				Args:    []string{"serve"},
			}))

			data2, err := os.ReadFile(mcpPath)
			require.NoError(t, err)

			var mcpDoc2 map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data2, &mcpDoc2))

			var servers2 map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(mcpDoc2["mcpServers"], &servers2))
			assert.Len(t, servers2, 1, "should still have exactly one server entry after idempotent merge")
		})
	}
}
