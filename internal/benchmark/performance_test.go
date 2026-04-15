//go:build benchmark

package benchmark

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestBuiltinScenariosPerformance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := NewRunner(logger)

	start := time.Now()
	_, err := runner.Run(context.Background(), BuiltinScenarios())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Runner.Run failed: %v", err)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("full suite took %v, want < 10s", elapsed)
	}
	t.Logf("full suite completed in %v", elapsed)
}
