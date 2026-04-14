package hook

import (
	"os"
	"path/filepath"
	"time"
)

// SessionState stores the state of an active hook session.
type SessionState struct {
	SessionID       string        `json:"session_id"`
	ProjectID       string        `json:"project_id"`
	StartedAt       time.Time     `json:"started_at"`
	LastActivity    time.Time     `json:"last_activity"`
	LastNudgeAt     time.Time     `json:"last_nudge_at,omitempty"`
	PID             int           `json:"pid"`
	InitialQuery    string        `json:"initial_query,omitempty"`
	ActiveTopic     string        `json:"active_topic,omitempty"`
	RecentSearches  []SearchEntry `json:"recent_searches"`
	StoresAttempted int           `json:"stores_attempted"`
	StoresSucceeded int           `json:"stores_succeeded"`
	StoredMemoryIDs []string      `json:"stored_memory_ids"`
}

// SearchEntry records a single search performed during a session.
type SearchEntry struct {
	Query     string    `json:"query"`
	Topic     string    `json:"topic"`
	Timestamp time.Time `json:"timestamp"`
}

// ResolveSessionDir returns the path to the sessions directory.
// Priority order:
//  1. Project-local: <projectDir>/.mnemos/<sessionDir>  (if projectDir is non-empty and writable)
//  2. Global fallback: ~/.mnemos/<sessionDir>
//  3. Relative fallback: .mnemos/<sessionDir>  (when home dir is unavailable, e.g. sandboxed CI)
func ResolveSessionDir(projectDir string, sessionDir string) string {
	// 1. Project-local path — preferred when a project dir is known
	if projectDir != "" {
		local := filepath.Join(projectDir, ".mnemos", sessionDir)
		if isWritableDir(local) {
			return local
		}
		// Try to create it — if that succeeds, it's usable
		if err := os.MkdirAll(local, 0o755); err == nil {
			return local
		}
	}

	// 2. Global ~/.mnemos/<sessionDir>
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".mnemos", sessionDir)
	}

	// 3. Relative fallback for sandboxed/containerized environments
	return filepath.Join(".mnemos", sessionDir)
}

// isWritableDir returns true if dir exists and is writable.
func isWritableDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return false
	}
	// Probe writability with a temp file
	tmp, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return false
	}
	tmp.Close()
	os.Remove(tmp.Name())
	return true
}
