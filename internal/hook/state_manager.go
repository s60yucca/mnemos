package hook

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
)

// StateManager manages session state files.
type StateManager struct {
	dir string // resolved session directory
	cfg *config.HookConfig
}

// NewStateManager creates a StateManager, resolving the session directory
// from projectDir and cfg.SessionDir.
func NewStateManager(projectDir string, cfg *config.HookConfig) *StateManager {
	return &StateManager{
		dir: ResolveSessionDir(projectDir, cfg.SessionDir),
		cfg: cfg,
	}
}

func (sm *StateManager) filePath(sessionID string) string {
	return filepath.Join(sm.dir, fmt.Sprintf("session-%s.json", sessionID))
}

// Get reads session state from file. Returns nil if not found or corrupt.
func (sm *StateManager) Get(sessionID string) *SessionState {
	data, err := os.ReadFile(sm.filePath(sessionID))
	if err != nil {
		return nil
	}
	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	return &state
}

// Save writes session state to file atomically (temp file + rename).
func (sm *StateManager) Save(state *SessionState) error {
	if err := os.MkdirAll(sm.dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal session state: %w", err)
	}

	tmp, err := os.CreateTemp(sm.dir, "session-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, sm.filePath(state.SessionID)); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// Delete removes the session state file, ignoring not-found errors.
func (sm *StateManager) Delete(sessionID string) error {
	err := os.Remove(sm.filePath(sessionID))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CleanStale removes stale, orphan, and expired session files.
func (sm *StateManager) CleanStale() error {
	entries, err := os.ReadDir(sm.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "session-") || !strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(sm.dir, name)

		info, err := entry.Info()
		if err != nil {
			slog.Warn("state_manager: failed to stat session file", "file", name, "err", err)
			continue
		}

		// Expired by file modification time (cleanup_retention)
		if sm.cfg.CleanupRetention > 0 && info.ModTime().Add(sm.cfg.CleanupRetention).Before(time.Now()) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				slog.Warn("state_manager: failed to remove expired session file", "file", name, "err", err)
			}
			continue
		}

		// Read and decode state for further checks
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("state_manager: failed to read session file", "file", name, "err", err)
			continue
		}

		var state SessionState
		if err := json.Unmarshal(data, &state); err != nil {
			// Corrupt file — remove it
			slog.Warn("state_manager: removing corrupt session file", "file", name, "err", err)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				slog.Warn("state_manager: failed to remove corrupt session file", "file", name, "err", err)
			}
			continue
		}

		// Stale by last_activity
		if sm.cfg.StaleTimeout > 0 && state.LastActivity.Add(sm.cfg.StaleTimeout).Before(time.Now()) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				slog.Warn("state_manager: failed to remove stale session file", "file", name, "err", err)
			}
			continue
		}

		// Orphan: PID is no longer running
		if state.PID > 0 && !isPIDRunning(state.PID) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				slog.Warn("state_manager: failed to remove orphan session file", "file", name, "err", err)
			}
			continue
		}
	}

	return nil
}

// isPIDRunning checks whether a process with the given PID is still alive.
func isPIDRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal(0) checks existence without sending a real signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
