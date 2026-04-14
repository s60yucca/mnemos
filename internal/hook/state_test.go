package hook_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/hook"
)

func TestSessionState_PathResolution(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	// Without a project dir, should fall back to ~/.mnemos/sessions
	got := hook.ResolveSessionDir("", "sessions")
	want := filepath.Join(home, ".mnemos", "sessions")

	if got != want {
		t.Errorf("ResolveSessionDir (no project) = %q, want %q", got, want)
	}
}

func TestSessionState_PathResolution_ProjectLocal(t *testing.T) {
	projectDir := t.TempDir()

	// With a writable project dir, should prefer project-local path
	got := hook.ResolveSessionDir(projectDir, "sessions")
	want := filepath.Join(projectDir, ".mnemos", "sessions")

	if got != want {
		t.Errorf("ResolveSessionDir (project local) = %q, want %q", got, want)
	}
}

func TestSessionState_PathResolution_FallbackWhenProjectNotWritable(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	// Non-existent project dir that can't be created (use a file path as parent)
	tmpFile, err := os.CreateTemp("", "not-a-dir-*")
	if err != nil {
		t.Skip("cannot create temp file")
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// projectDir is a file, not a directory — MkdirAll will fail
	got := hook.ResolveSessionDir(tmpFile.Name(), "sessions")
	// Should fall back to global ~/.mnemos/sessions
	want := filepath.Join(home, ".mnemos", "sessions")

	if got != want {
		t.Errorf("ResolveSessionDir (unwritable project) = %q, want %q", got, want)
	}
}

func newTestStateManager(t *testing.T) *hook.StateManager {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "sessions")

	cfg := &config.HookConfig{
		SessionDir: "sessions",
	}
	return hook.NewStateManagerWithDir(dir, cfg)
}

func TestStateManager_SaveAndGet(t *testing.T) {
	sm := newTestStateManager(t)

	state := &hook.SessionState{
		SessionID:    "test-session-1",
		ProjectID:    "proj-abc",
		StartedAt:    time.Now().Truncate(time.Second),
		LastActivity: time.Now().Truncate(time.Second),
		PID:          os.Getpid(),
	}

	if err := sm.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got := sm.Get(state.SessionID)
	if got == nil {
		t.Fatal("Get returned nil after Save")
	}
	if got.SessionID != state.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, state.SessionID)
	}
	if got.ProjectID != state.ProjectID {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, state.ProjectID)
	}
}

func TestStateManager_Get_NotFound(t *testing.T) {
	sm := newTestStateManager(t)

	got := sm.Get("nonexistent-session")
	if got != nil {
		t.Errorf("expected nil for missing session, got %+v", got)
	}
}

func TestStateManager_Delete(t *testing.T) {
	sm := newTestStateManager(t)

	state := &hook.SessionState{
		SessionID:    "delete-me",
		ProjectID:    "proj-xyz",
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
		PID:          os.Getpid(),
	}

	if err := sm.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := sm.Delete(state.SessionID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	got := sm.Get(state.SessionID)
	if got != nil {
		t.Error("expected nil after Delete, but Get returned a state")
	}
}

func TestStateManager_Delete_NotFound(t *testing.T) {
	sm := newTestStateManager(t)

	// Deleting a non-existent session should not error
	if err := sm.Delete("ghost-session"); err != nil {
		t.Errorf("Delete of non-existent session returned error: %v", err)
	}
}
