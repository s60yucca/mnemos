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
// If .mnemos/ exists in projectDir, returns projectDir/.mnemos/<sessionDir>.
// Otherwise returns ~/.mnemos/<sessionDir>.
func ResolveSessionDir(projectDir string, sessionDir string) string {
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	if info, err := os.Stat(mnemosDir); err == nil && info.IsDir() {
		return filepath.Join(mnemosDir, sessionDir)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		// fallback to relative path if home dir is unavailable
		return filepath.Join(".mnemos", sessionDir)
	}
	return filepath.Join(home, ".mnemos", sessionDir)
}
