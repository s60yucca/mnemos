package autopilot

import (
	"context"
	"log/slog"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

const FindingBackfillCompleted FindingType = "backfill_completed"

// SummaryBackfillDetector finds active memories with empty summaries and
// populates them using ExtractSummary.
type SummaryBackfillDetector struct {
	mnemos *core.Mnemos
	cfg    config.AutopilotConfig
	logger *slog.Logger
}

// NewSummaryBackfillDetector creates a new SummaryBackfillDetector.
func NewSummaryBackfillDetector(mnemos *core.Mnemos, cfg config.AutopilotConfig, logger *slog.Logger) *SummaryBackfillDetector {
	return &SummaryBackfillDetector{mnemos: mnemos, cfg: cfg, logger: logger}
}

// Name returns the detector name.
func (d *SummaryBackfillDetector) Name() string { return "summary_backfill" }

// Detect queries memories with empty summaries and backfills them.
func (d *SummaryBackfillDetector) Detect(ctx context.Context, projectID string, maps EntityMaps) ([]Finding, error) {
	limit := d.cfg.MaxBackfillPerRun
	if limit <= 0 {
		limit = 50
	}

	memories, err := d.mnemos.List(ctx, storage.ListQuery{
		ProjectID:    projectID,
		Statuses:     []domain.MemoryStatus{domain.MemoryStatusActive},
		EmptySummary: true,
		SortBy:       "created_at",
		SortDesc:     false,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}

	count := 0
	for _, mem := range memories {
		summary := coremem.ExtractSummary(mem.Content, mem.Type, 30)
		if summary == "" {
			continue
		}
		if _, err := d.mnemos.Update(ctx, &domain.UpdateRequest{
			ID:      mem.ID,
			Summary: &summary,
		}); err != nil {
			d.logger.Warn("backfill: update failed", "id", mem.ID, "err", err)
			continue
		}
		count++
	}

	if count == 0 {
		return nil, nil
	}
	return []Finding{{
		Type:     FindingBackfillCompleted,
		Metadata: map[string]any{"count": count},
	}}, nil
}
