package hook_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestProjectDetector_EnvVar(t *testing.T) {
	t.Setenv("MNEMOS_PROJECT_ID", "test-env-project")

	d := hook.NewProjectDetector("/some/dir", "/some/data")
	projectID, strategy, err := d.Detect()

	require.NoError(t, err)
	assert.Equal(t, "test-env-project", projectID)
	assert.Equal(t, "env_var", strategy)
}

func TestProjectDetector_DirBasenameFallback(t *testing.T) {
	// Ensure env var is empty
	t.Setenv("MNEMOS_PROJECT_ID", "")

	// Since findGitRoot will fail or return empty if we're not in a git repo,
	// let's use a temp dir that is definitely not a git repo.
	tempDir := t.TempDir()
	d := hook.NewProjectDetector(tempDir, "/some/data")

	projectID, strategy, err := d.Detect()
	require.NoError(t, err)

	expectedBase := filepath.Base(tempDir)
	assert.Equal(t, expectedBase, projectID)
	assert.Equal(t, "dir_basename", strategy)
}

func TestProjectDetector_GitRemote(t *testing.T) {
	// Ensure env var is empty
	t.Setenv("MNEMOS_PROJECT_ID", "")

	tempDir := t.TempDir()

	// Initialize a dummy git repo and add a remote
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("test"), 0644))

	// We need git command to be available. If not, we skip.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed, skipping git remote test")
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "remote", "add", "origin", "git@github.com:mnemos-dev/mnemos-test.git")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	d := hook.NewProjectDetector(tempDir, "/some/data")
	projectID, strategy, err := d.Detect()

	require.NoError(t, err)
	assert.Equal(t, "mnemos-test", projectID)
	assert.Equal(t, "git_remote", strategy)
}

func TestAutoInjectConfigFromEnv(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("MNEMOS_AUTO_INJECT", "")
		t.Setenv("MNEMOS_AUTO_INJECT_BUDGET", "")
		t.Setenv("MNEMOS_AUTO_INJECT_COUNT", "")
		t.Setenv("MNEMOS_AUTO_INJECT_SUMMARY_LENGTH", "")

		cfg := hook.AutoInjectConfigFromEnv()
		assert.True(t, cfg.Enabled)
		assert.Equal(t, 1500, cfg.Budget)
		assert.Equal(t, 10, cfg.MaxCount)
		assert.Equal(t, 120, cfg.SummaryLength)
	})

	t.Run("custom values", func(t *testing.T) {
		t.Setenv("MNEMOS_AUTO_INJECT", "false")
		t.Setenv("MNEMOS_AUTO_INJECT_BUDGET", "2000")
		t.Setenv("MNEMOS_AUTO_INJECT_COUNT", "5")
		t.Setenv("MNEMOS_AUTO_INJECT_SUMMARY_LENGTH", "200")

		cfg := hook.AutoInjectConfigFromEnv()
		assert.False(t, cfg.Enabled)
		assert.Equal(t, 2000, cfg.Budget)
		assert.Equal(t, 5, cfg.MaxCount)
		assert.Equal(t, 200, cfg.SummaryLength)
	})

	t.Run("invalid values ignored", func(t *testing.T) {
		t.Setenv("MNEMOS_AUTO_INJECT_BUDGET", "invalid")
		t.Setenv("MNEMOS_AUTO_INJECT_COUNT", "-5")
		t.Setenv("MNEMOS_AUTO_INJECT_SUMMARY_LENGTH", "0")

		cfg := hook.AutoInjectConfigFromEnv()
		assert.Equal(t, 1500, cfg.Budget)
		assert.Equal(t, 10, cfg.MaxCount)
		assert.Equal(t, 120, cfg.SummaryLength)
	})
}

func TestFormatAutoInjectPayload_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		projectID := rapid.StringMatching(`[a-zA-Z0-9_-]+`).Draw(rt, "projectID")
		budget := rapid.IntRange(1, 10000).Draw(rt, "budget")
		maxCount := rapid.IntRange(1, 100).Draw(rt, "maxCount")
		summaryLen := rapid.IntRange(10, 500).Draw(rt, "summaryLen")

		cfg := hook.AutoInjectConfig{
			Enabled:       true,
			Budget:        budget,
			MaxCount:      maxCount,
			SummaryLength: summaryLen,
		}

		numMemories := rapid.IntRange(0, 20).Draw(rt, "numMemories")
		var memories []*domain.Memory

		for i := 0; i < numMemories; i++ {
			id := rapid.StringMatching(`[a-f0-9]{8}`).Draw(rt, "id")
			typ := rapid.SampledFrom([]domain.MemoryType{
				domain.MemoryTypeSemantic, domain.MemoryTypeEpisodic, domain.MemoryTypeSkill,
			}).Draw(rt, "type")
			category := rapid.StringMatching(`[a-z]+`).Draw(rt, "category")
			content := rapid.StringMatching(`[a-zA-Z0-9 ]{0,1000}`).Draw(rt, "content")
			summary := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "summary")

			// Occasionally make summary empty
			if rapid.Bool().Draw(rt, "emptySummary") {
				summary = ""
			}

			// Occasionally make category empty
			if rapid.Bool().Draw(rt, "emptyCategory") {
				category = ""
			}

			mem := &domain.Memory{
				ID:        id,
				Type:      typ,
				Category:  category,
				Content:   content,
				Summary:   summary,
				CreatedAt: time.Now(),
			}
			memories = append(memories, mem)
		}

		payload := hook.FormatAutoInjectPayloadForTest(memories, projectID, cfg)

		// Basic assertions
		assert.Contains(t, payload, "# Mnemos Project Context")
		assert.Contains(t, payload, projectID)
		assert.Contains(t, payload, "Use mnemos_get(<id>)")

		for _, mem := range memories {
			assert.Contains(t, payload, mem.ID)
			assert.Contains(t, payload, string(mem.Type))

			if mem.Category == "" {
				assert.Contains(t, payload, "other")
			} else {
				assert.Contains(t, payload, mem.Category)
			}

			if mem.Summary != "" {
				assert.Contains(t, payload, mem.Summary)
			} else {
				expectedContent := mem.Content
				if len(expectedContent) > cfg.SummaryLength {
					expectedContent = expectedContent[:cfg.SummaryLength]
				}
				assert.Contains(t, payload, expectedContent)
			}
		}
	})
}

func TestAutoInjector_Disabled(t *testing.T) {
	mn := newTestMnemos(t) // from dispatcher_test.go
	cfg := hook.AutoInjectConfig{Enabled: false}

	injector := hook.NewAutoInjector(mn, cfg, t.TempDir())

	payload, skipReason, err := injector.Run(context.Background(), "session-1", "proj-1", "client-1", nil)
	require.NoError(t, err)
	assert.Empty(t, payload)
	assert.Equal(t, "disabled", skipReason)
}

func TestAutoInjector_BenchModeOff(t *testing.T) {
	mn := newTestMnemos(t)
	cfg := hook.AutoInjectConfig{Enabled: true}

	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "bench_mode"), []byte("off"), 0644))

	injector := hook.NewAutoInjector(mn, cfg, dataDir)

	payload, skipReason, err := injector.Run(context.Background(), "session-1", "proj-1", "client-1", nil)
	require.NoError(t, err)
	assert.Empty(t, payload)
	assert.Equal(t, "bench_mode_off", skipReason)
}

func TestAutoInjector_Success(t *testing.T) {
	mn := newTestMnemos(t)
	cfg := hook.AutoInjectConfig{
		Enabled:       true,
		Budget:        1500,
		MaxCount:      10,
		SummaryLength: 120,
	}

	ctx := context.Background()
	projectID := "proj-test"

	// Store some memories that will match "current project context overview"
	_, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "current project context overview architecture design",
		Summary:   "test summary 1",
		Type:      domain.MemoryTypeSemantic,
		Category:  "architecture",
		ProjectID: projectID,
	})
	require.NoError(t, err)

	// Store a memory with bench_off_day tag to ensure it gets filtered
	_, err = mn.StoreWithoutGate(ctx, &domain.StoreRequest{
		Content:   "test bench off day",
		Type:      domain.MemoryTypeSemantic,
		Tags:      []string{"bench_off_day"},
		ProjectID: projectID,
	})
	require.NoError(t, err)

	injector := hook.NewAutoInjector(mn, cfg, t.TempDir())

	payload, skipReason, err := injector.Run(ctx, "session-1", projectID, "client-1", nil)
	require.NoError(t, err)

	assert.Empty(t, skipReason)
	assert.NotEmpty(t, payload)

	assert.Contains(t, payload, "test summary 1")
	assert.NotContains(t, payload, "test bench off day")
}
