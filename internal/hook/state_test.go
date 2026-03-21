package hook_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/hook"
)

func TestSessionState_PathResolution_WithMnemosDir(t *testing.T) {
	projectDir := t.TempDir()

	// Create .mnemos/ directory inside projectDir
	mnemosDir := filepath.Join(projectDir, ".mnemos")
	if err := os.Mkdir(mnemosDir, 0o755); err != nil {
		t.Fatalf("failed to create .mnemos dir: %v", err)
	}

	got := hook.ResolveSessionDir(projectDir, "sessions")
	want := filepath.Join(mnemosDir, "sessions")

	if got != want {
		t.Errorf("ResolveSessionDir = %q, want %q", got, want)
	}
}

func TestSessionState_PathResolution_Fallback(t *testing.T) {
	projectDir := t.TempDir()
	// No .mnemos/ directory created — should fall back to ~/.mnemos/sessions

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	got := hook.ResolveSessionDir(projectDir, "sessions")
	want := filepath.Join(home, ".mnemos", "sessions")

	if got != want {
		t.Errorf("ResolveSessionDir = %q, want %q", got, want)
	}
}

func newTestStateManager(t *testing.T) *hook.StateManager {
	t.Helper()
	dir := t.TempDir()

	// Create .mnemos inside the temp dir so ResolveSessionDir uses it
	mnemosDir := filepath.Join(dir, ".mnemos")
	if err := os.Mkdir(mnemosDir, 0o755); err != nil {
		t.Fatalf("failed to create .mnemos dir: %v", err)
	}

	cfg := &config.HookConfig{
		SessionDir: "sessions",
	}
	return hook.NewStateManager(dir, cfg)
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
