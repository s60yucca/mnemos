package main

import (
	"context"
	"fmt"

	core "github.com/mnemos-dev/mnemos/internal/core"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/spf13/cobra"
)

// newBackfillCmd creates the "mnemos backfill" parent command.
func newBackfillCmd(mnemos *core.Mnemos) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill missing data for existing memories",
	}

	cmd.AddCommand(newBackfillSummariesCmd(mnemos))

	return cmd
}

// newBackfillSummariesCmd creates the "mnemos backfill summaries" subcommand.
func newBackfillSummariesCmd(mnemos *core.Mnemos) *cobra.Command {
	var projectID string
	var dryRun bool
	var limit int

	cmd := &cobra.Command{
		Use:   "summaries",
		Short: "Backfill empty summaries for existing memories",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			query := storage.ListQuery{
				ProjectID:    projectID,
				Statuses:     []domain.MemoryStatus{domain.MemoryStatusActive},
				EmptySummary: true,
				SortBy:       "created_at",
				SortDesc:     false,
			}
			if limit == 0 {
				query.Limit = 100000 // effectively all
			} else {
				query.Limit = limit
			}

			memories, err := mnemos.List(ctx, query)
			if err != nil {
				return fmt.Errorf("error listing memories: %w", err)
			}

			count := 0
			for _, mem := range memories {
				summary := coremem.ExtractSummary(mem.Content, mem.Type, 30)
				if summary == "" {
					continue
				}
				if dryRun {
					fmt.Printf("[DRY RUN] would update %s: %s\n", mem.ID, summary)
					count++
					continue
				}
				if _, err := mnemos.Update(ctx, &domain.UpdateRequest{
					ID:      mem.ID,
					Summary: &summary,
				}); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "error updating %s: %v\n", mem.ID, err)
					continue
				}
				count++
			}

			if dryRun {
				fmt.Printf("[DRY RUN] would update %d memories\n", count)
			} else {
				fmt.Printf("updated %d memories\n", count)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "project ID to backfill (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be updated without writing")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap number of memories processed (0 = all)")
	cmd.MarkFlagRequired("project") //nolint:errcheck

	return cmd
}
