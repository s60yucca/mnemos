package setup

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Writer handles writing setup files to the filesystem.
type Writer struct {
	projectDir string
	global     bool
	force      bool
	logger     *slog.Logger
	written    []string // track written files for Report()
}

// NewWriter creates a new Writer.
func NewWriter(projectDir string, global bool, force bool) *Writer {
	return &Writer{
		projectDir: projectDir,
		global:     global,
		force:      force,
		logger:     slog.Default(),
	}
}

// WriteFile writes a file from template content, asking for confirmation if the
// file already exists (unless force=true). Uses atomic write: temp file + rename.
// Returns (written bool, err error).
func (w *Writer) WriteFile(targetPath, templateContent string) (bool, error) {
	if _, err := os.Stat(targetPath); err == nil {
		// File exists
		if !w.force {
			confirmed, err := w.confirmOverwrite(targetPath)
			if err != nil {
				return false, err
			}
			if !confirmed {
				return false, nil
			}
		}
	}

	if err := w.atomicWrite(targetPath, templateContent); err != nil {
		return false, err
	}

	w.written = append(w.written, targetPath)
	return true, nil
}

// EnsureDir creates a directory if it doesn't exist.
func (w *Writer) EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// Report prints the list of files created/updated to stdout.
func (w *Writer) Report() {
	for _, f := range w.written {
		fmt.Printf("Created: %s\n", f)
	}
}

// confirmOverwrite prompts the user to confirm overwriting an existing file.
func (w *Writer) confirmOverwrite(path string) (bool, error) {
	fmt.Printf("File %s already exists. Overwrite? [y/N]: ", path)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// atomicWrite writes content to path using a temp file + rename for atomicity.
func (w *Writer) atomicWrite(targetPath, content string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".mnemos-write-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Clean up temp file on failure
	success := false
	defer func() {
		if !success {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	success = true
	return nil
}

// EnsureGlobalConfig creates ~/.mnemos/ and a default config.yaml if not present.
// Returns the data directory path. Safe to call multiple times (idempotent).
func EnsureGlobalConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	dataDir := filepath.Join(home, ".mnemos")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	cfgPath := filepath.Join(dataDir, "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		defaultCfg := fmt.Sprintf(`# Mnemos configuration — shared by all AI clients
# See https://github.com/mnemos-dev/mnemos for documentation
# Per-project overrides: use MNEMOS_* environment variables

data_dir: %s
log_level: info

embeddings:
  enabled: false
  provider: noop
  # Uncomment for semantic search:
  # provider: ollama
  # base_url: http://localhost:11434
  # model: nomic-embed-text
  # dims: 768

hook:
  enabled: true
  search_cooldown: 5m
  session_start_max_tokens: 2000

mirror:
  enabled: true

lifecycle:
  gc_retention_days: 30
`, dataDir)
		if err := os.WriteFile(cfgPath, []byte(defaultCfg), 0o644); err != nil {
			return "", fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("Created config: %s\n", cfgPath)
	}
	return dataDir, nil
}
