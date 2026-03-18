package markdown

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

// Mirror implements storage.IMarkdownMirror
type Mirror struct {
	basePath string
	enabled  bool
}

func NewMirror(basePath string, enabled bool) *Mirror {
	return &Mirror{basePath: basePath, enabled: enabled}
}

func (m *Mirror) IsEnabled() bool  { return m.enabled }
func (m *Mirror) GetBasePath() string { return m.basePath }

func (m *Mirror) SyncMemory(_ context.Context, mem *domain.Memory) error {
	if !m.enabled {
		return nil
	}
	path := m.memoryPath(mem)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	content := renderMarkdown(mem)
	return os.WriteFile(path, []byte(content), 0644)
}

func (m *Mirror) SyncRelation(_ context.Context, _ *domain.MemoryRelation) error {
	return nil // relations not mirrored as separate files
}

func (m *Mirror) DeleteMemory(_ context.Context, id string) error {
	if !m.enabled {
		return nil
	}
	// Search for the file by ID pattern
	return filepath.Walk(m.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), id+".md") {
			os.Remove(path) //nolint:errcheck
		}
		return nil
	})
}

func (m *Mirror) SyncAll(_ context.Context, memories []*domain.Memory) error {
	if !m.enabled {
		return nil
	}
	for _, mem := range memories {
		path := m.memoryPath(mem)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			continue
		}
		os.WriteFile(path, []byte(renderMarkdown(mem)), 0644) //nolint:errcheck
	}
	return nil
}

func (m *Mirror) memoryPath(mem *domain.Memory) string {
	project := mem.ProjectID
	if project == "" {
		project = "global"
	}
	return filepath.Join(m.basePath, project, mem.Category, mem.ID+".md")
}

func renderMarkdown(mem *domain.Memory) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", mem.ID))
	sb.WriteString(fmt.Sprintf("type: %s\n", mem.Type))
	sb.WriteString(fmt.Sprintf("category: %s\n", mem.Category))
	if len(mem.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(mem.Tags, ", ")))
	}
	if mem.Source != "" {
		sb.WriteString(fmt.Sprintf("source: %s\n", mem.Source))
	}
	sb.WriteString(fmt.Sprintf("created_at: %s\n", mem.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("relevance_score: %.4f\n", mem.RelevanceScore))
	sb.WriteString(fmt.Sprintf("status: %s\n", mem.Status))
	sb.WriteString("---\n\n")
	sb.WriteString(mem.Content)
	sb.WriteString("\n")
	return sb.String()
}
