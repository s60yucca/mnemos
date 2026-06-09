package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/oklog/ulid/v2"
)

// BenchMode represents whether mnemos autopilot is active during benchmark sessions.
type BenchMode string

const (
	BenchModeOn    BenchMode = "on"
	BenchModeOff   BenchMode = "off"
	BenchModeMixed BenchMode = "mode_mixed"
)

// Session represents one Kiro task execution for benchmark tracking.
type Session struct {
	ID            string
	ProjectID     string
	Mode          BenchMode
	StartTime     time.Time
	EndTime       time.Time
	TokensIn      int
	TokensOut     int
	MCPCallsCount int
	TaskCompleted bool
	TaskCategory  string
	Provenance    string
}

// SessionTracker manages benchmark session tracking with automatic timeout detection.
type SessionTracker struct {
	mu                sync.Mutex
	currentSession    *Session
	lastActivity      time.Time
	tokenCounter      *TokenCounter
	benchMode         BenchMode
	dataDir           string
	ticker            *time.Ticker
	stopChan          chan struct{}
	wg                sync.WaitGroup
	stopped           bool
	inactivityTimeout time.Duration
	provenance        string
}

// NewSessionTracker creates a new session tracker with default 10-minute timeout.
func NewSessionTracker(dataDir string) (*SessionTracker, error) {
	return newSessionTracker(dataDir, 10*time.Minute, benchmarkProvenance("production"))
}

// NewSessionTrackerWithTimeout creates a new session tracker with custom timeout.
// This is primarily for testing - production code should use NewSessionTracker.
func NewSessionTrackerWithTimeout(dataDir string, inactivityTimeout time.Duration) (*SessionTracker, error) {
	return newSessionTracker(dataDir, inactivityTimeout, benchmarkProvenance("test"))
}

func newSessionTracker(dataDir string, inactivityTimeout time.Duration, provenance string) (*SessionTracker, error) {
	counter, err := NewTokenCounter()
	if err != nil {
		return nil, fmt.Errorf("failed to create token counter: %w", err)
	}

	// Read current bench mode
	mode, err := ReadBenchMode(dataDir)
	if err != nil {
		// Default to ON if file doesn't exist
		mode = BenchModeOn
	}

	tracker := &SessionTracker{
		tokenCounter:      counter,
		benchMode:         mode,
		dataDir:           dataDir,
		stopChan:          make(chan struct{}),
		inactivityTimeout: inactivityTimeout,
		provenance:        provenance,
	}

	// Start inactivity timeout checker
	tracker.startTimeoutChecker()

	return tracker, nil
}

// OnMCPCall tracks MCP activity and accumulates token counts.
func (t *SessionTracker) OnMCPCall(projectID string, reqContent string, respContent string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	currentMode, err := ReadBenchMode(t.dataDir)
	if err == nil {
		t.benchMode = currentMode
	}

	// Start new session if none exists
	if t.currentSession == nil {
		t.startSessionLocked(projectID, "other")
	} else if t.currentSession.ProjectID == "" && projectID != "" {
		t.currentSession.ProjectID = projectID
	} else if projectID != "" && t.currentSession.ProjectID != projectID {
		t.endSessionLocked(false)
		t.startSessionLocked(projectID, "other")
	} else if t.currentSession.Mode != t.benchMode && t.currentSession.Mode != BenchModeMixed {
		t.currentSession.Mode = BenchModeMixed
	}

	// Update last activity
	t.lastActivity = time.Now()

	// Accumulate token counts
	t.currentSession.TokensIn += t.tokenCounter.CountTokens(reqContent)
	t.currentSession.TokensOut += t.tokenCounter.CountTokens(respContent)
	t.currentSession.MCPCallsCount++
}

// StartSession begins a new benchmark session with optional category.
func (t *SessionTracker) StartSession(projectID string, category string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// End current session if exists
	if t.currentSession != nil {
		t.endSessionLocked(false)
	}

	t.startSessionLocked(projectID, category)
}

// startSessionLocked starts a new session (must be called with lock held).
func (t *SessionTracker) startSessionLocked(projectID string, category string) {
	if category == "" {
		category = "other"
	}

	session := &Session{
		ID:           ulid.Make().String(),
		ProjectID:    projectID,
		Mode:         t.benchMode,
		StartTime:    time.Now(),
		TaskCategory: category,
		Provenance:   t.provenance,
	}

	t.currentSession = session
	t.lastActivity = time.Now()

	// Emit session_start event
	observe.Feature("bench_session_start", map[string]any{
		"session_id": session.ID,
		"project_id": session.ProjectID,
		"mode":       string(session.Mode),
		"category":   session.TaskCategory,
		"timestamp":  session.StartTime.Format(time.RFC3339),
		"provenance": session.Provenance,
	})
}

func benchmarkProvenance(defaultValue string) string {
	switch value := os.Getenv("MNEMOS_BENCH_PROVENANCE"); value {
	case "test", "fixture", "synthetic", "dry-run", "production":
		return value
	default:
		return defaultValue
	}
}

// EndSession finalizes the current session.
func (t *SessionTracker) EndSession(taskCompleted bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentSession == nil {
		return
	}

	t.endSessionLocked(taskCompleted)
}

// endSessionLocked ends the current session (must be called with lock held).
func (t *SessionTracker) endSessionLocked(taskCompleted bool) {
	if t.currentSession == nil {
		return
	}

	t.currentSession.EndTime = time.Now()
	t.currentSession.TaskCompleted = taskCompleted

	duration := t.currentSession.EndTime.Sub(t.currentSession.StartTime)

	// Emit session_end event
	observe.Feature("bench_session_end", map[string]any{
		"session_id":      t.currentSession.ID,
		"duration_ms":     duration.Milliseconds(),
		"tokens_in":       t.currentSession.TokensIn,
		"tokens_out":      t.currentSession.TokensOut,
		"mcp_calls_count": t.currentSession.MCPCallsCount,
		"task_completed":  taskCompleted,
		"task_category":   t.currentSession.TaskCategory,
		"mode":            string(t.currentSession.Mode),
	})

	t.currentSession = nil
}

// GetCurrentSession returns the current session (if any).
func (t *SessionTracker) GetCurrentSession() *Session {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentSession == nil {
		return nil
	}

	// Return a copy to avoid race conditions
	sessionCopy := *t.currentSession
	return &sessionCopy
}

// startTimeoutChecker starts a background goroutine to check for inactivity.
func (t *SessionTracker) startTimeoutChecker() {
	// Use a ticker interval that's 1/10th of the timeout, with a minimum of 100ms
	tickerInterval := t.inactivityTimeout / 10
	if tickerInterval < 100*time.Millisecond {
		tickerInterval = 100 * time.Millisecond
	}

	t.ticker = time.NewTicker(tickerInterval)
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ticker.C:
				t.checkInactivity()
			case <-t.stopChan:
				return
			}
		}
	}()
}

// checkInactivity checks if the current session has been inactive for too long.
func (t *SessionTracker) checkInactivity() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentSession == nil {
		return
	}

	// Check if inactive for more than the configured timeout
	if time.Since(t.lastActivity) > t.inactivityTimeout {
		t.endSessionLocked(true)
	}
}

// Stop gracefully shuts down the session tracker.
func (t *SessionTracker) Stop() {
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return
	}
	t.stopped = true
	t.mu.Unlock()

	// Stop the timeout checker
	close(t.stopChan)
	if t.ticker != nil {
		t.ticker.Stop()
	}
	t.wg.Wait()

	// End current session if exists
	t.EndSession(false)
}

// ReadBenchMode reads the benchmark mode from ~/.mnemos/bench_mode.
func ReadBenchMode(dataDir string) (BenchMode, error) {
	path := filepath.Join(dataDir, "bench_mode")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return BenchModeOn, nil // Default to ON
		}
		return "", fmt.Errorf("failed to read bench mode: %w", err)
	}

	mode := BenchMode(string(data))
	if mode != BenchModeOn && mode != BenchModeOff {
		return BenchModeOn, nil // Default to ON if invalid
	}

	return mode, nil
}

// WriteBenchMode writes the benchmark mode to ~/.mnemos/bench_mode.
func WriteBenchMode(dataDir string, mode BenchMode) error {
	if mode != BenchModeOn && mode != BenchModeOff {
		return fmt.Errorf("invalid bench mode: %s", mode)
	}

	path := filepath.Join(dataDir, "bench_mode")

	// Create parent directory if needed
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(mode), 0644); err != nil {
		return fmt.Errorf("failed to write bench mode: %w", err)
	}

	return nil
}
