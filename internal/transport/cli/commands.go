package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/setup"
	"github.com/mnemos-dev/mnemos/internal/storage"
	mcptransport "github.com/mnemos-dev/mnemos/internal/transport/mcp"
	"github.com/spf13/cobra"
)

func newJSONEncoder(w io.Writer) *json.Encoder {
	return json.NewEncoder(w)
}

func newStoreCmd(m *core.Mnemos) *cobra.Command {
	var memType, category, tags, source, summary string
	cmd := &cobra.Command{
		Use:   "store <content>",
		Short: "Store a new memory",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content := strings.Join(args, " ")
			req := &domain.StoreRequest{
				Content:   content,
				Summary:   summary,
				Source:    source,
				ProjectID: projectID,
			}
			if memType != "" {
				req.Type = domain.MemoryType(memType)
			}
			if category != "" {
				req.Category = category
			}
			if tags != "" {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						req.Tags = append(req.Tags, t)
					}
				}
			}
			result, err := m.Store(context.Background(), req)
			if err != nil {
				return err
			}
			printJSON(result)
			return nil
		},
	}
	cmd.Flags().StringVarP(&memType, "type", "t", "", "memory type")
	cmd.Flags().StringVarP(&category, "category", "c", "", "category")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	cmd.Flags().StringVar(&source, "source", "", "source identifier")
	cmd.Flags().StringVar(&summary, "summary", "", "optional summary")
	return cmd
}

func newGetCmd(m *core.Mnemos) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a memory by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mem, err := m.Get(context.Background(), args[0])
			if err != nil {
				return err
			}
			printJSON(mem)
			return nil
		},
	}
}

func newListCmd(m *core.Mnemos) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List memories",
		RunE: func(cmd *cobra.Command, args []string) error {
			memories, err := m.List(context.Background(), storage.ListQuery{
				ProjectID: projectID,
				Limit:     limit,
				SortBy:    "created_at",
				SortDesc:  true,
			})
			if err != nil {
				return err
			}
			// Table output
			fmt.Printf("%-26s %-12s %-14s %-8s %s\n", "ID", "TYPE", "CATEGORY", "SCORE", "CONTENT")
			fmt.Println(strings.Repeat("-", 90))
			for _, mem := range memories {
				preview := mem.ContentPreview(40)
				fmt.Printf("%-26s %-12s %-14s %-8.3f %s\n",
					mem.ID, mem.Type, mem.Category, mem.RelevanceScore, preview)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "max results")
	return cmd
}

func newSearchCmd(m *core.Mnemos) *cobra.Command {
	var limit int
	var mode string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search memories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			var results []*storage.SearchResult
			var err error

			switch mode {
			case "text":
				results, err = m.TextSearch(context.Background(), storage.TextSearchQuery{
					Query:     query,
					ProjectID: projectID,
					Limit:     limit,
				})
			case "semantic":
				results, err = m.SemanticSearch(context.Background(), query, projectID, limit, 0.5)
			case "", "hybrid":
				results, err = m.Search(context.Background(), query, projectID, limit)
			default:
				return fmt.Errorf("mode must be one of: text, semantic, hybrid")
			}
			if err != nil {
				return err
			}

			fmt.Printf("%-26s %-8s %-8s %s\n", "ID", "SCORE", "SOURCE", "CONTENT")
			fmt.Println(strings.Repeat("-", 80))
			for _, r := range results {
				score := r.HybridScore
				if score == 0 {
					score = r.TextScore
				}
				fmt.Printf("%-26s %-8.4f %-8s %s\n",
					r.Memory.ID, score, r.Source, r.Memory.ContentPreview(40))
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "max results")
	cmd.Flags().StringVar(&mode, "mode", "hybrid", "search mode: text|semantic|hybrid")
	return cmd
}

func newUpdateCmd(m *core.Mnemos) *cobra.Command {
	var content, summary, tags string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := &domain.UpdateRequest{ID: args[0]}
			if content != "" {
				req.Content = &content
			}
			if summary != "" {
				req.Summary = &summary
			}
			if tags != "" {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						req.Tags = append(req.Tags, t)
					}
				}
			}
			mem, err := m.Update(context.Background(), req)
			if err != nil {
				return err
			}
			printJSON(mem)
			return nil
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "new content")
	cmd.Flags().StringVar(&summary, "summary", "", "new summary")
	cmd.Flags().StringVar(&tags, "tags", "", "new comma-separated tags")
	return cmd
}

func newDeleteCmd(m *core.Mnemos) *cobra.Command {
	var hard bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if hard {
				err = m.HardDelete(context.Background(), args[0])
			} else {
				err = m.Delete(context.Background(), args[0])
			}
			if err != nil {
				return err
			}
			fmt.Printf("deleted: %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&hard, "hard", false, "permanently delete")
	return cmd
}

func newRelateCmd(m *core.Mnemos) *cobra.Command {
	var relType string
	var strength float64
	cmd := &cobra.Command{
		Use:   "relate <source-id> <target-id>",
		Short: "Create a relation between two memories",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strength == 0 {
				strength = 1.0
			}
			rel, err := m.Relate(context.Background(), &domain.RelateRequest{
				SourceID:     args[0],
				TargetID:     args[1],
				RelationType: domain.RelationType(relType),
				Strength:     strength,
			})
			if err != nil {
				return err
			}
			printJSON(rel)
			return nil
		},
	}
	cmd.Flags().StringVarP(&relType, "type", "t", "relates_to", "relation type")
	cmd.Flags().Float64Var(&strength, "strength", 1.0, "relation strength [0.0, 1.0]")
	return cmd
}

func newStatsCmd(m *core.Mnemos) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show storage statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			stats, err := m.Stats(context.Background(), projectID)
			if err != nil {
				return err
			}
			printJSON(stats)
			return nil
		},
	}
}

func newMaintainCmd(m *core.Mnemos) *cobra.Command {
	return &cobra.Command{
		Use:   "maintain",
		Short: "Run decay, archival, and GC",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Running maintenance...")
			if err := m.Maintain(context.Background(), projectID); err != nil {
				return err
			}
			fmt.Println("Maintenance complete.")
			return nil
		},
	}
}

func newServeCmd(m *core.Mnemos, version string) *cobra.Command {
	var rest bool
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start MCP or REST server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rest {
				// REST server handled in main wiring
				fmt.Printf("REST server not yet wired in serve command; use --rest flag with main\n")
				return nil
			}
			// MCP stdio mode
			mcpServer := mcptransport.NewServer(m, version)
			return mcpServer.ServeStdio(context.Background())
		},
	}
	cmd.Flags().BoolVar(&rest, "rest", false, "start REST server instead of MCP")
	cmd.Flags().IntVar(&port, "port", 8080, "REST server port")
	return cmd
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Mnemos data directory and config",
		Long: `Create ~/.mnemos/ with default config.yaml and data directory.

The global config is shared by all AI clients (Kiro, Claude, Cursor).
Per-project overrides are available via MNEMOS_* environment variables.

Examples:
  mnemos init
  MNEMOS_EMBEDDINGS_PROVIDER=ollama mnemos serve`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := setup.EnsureGlobalConfig()
			if err != nil {
				return err
			}
			fmt.Printf("Initialized Mnemos at %s\n", dataDir)
			return nil
		},
	}
}

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("mnemos", version)
		},
	}
}
