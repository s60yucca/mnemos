package hook

import (
	"context"

	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/search"
)

// FormatContextResultForTest exposes formatContextResult for external test packages.
func FormatContextResultForTest(ctx context.Context, m *core.Mnemos, projectID string, result *search.ContextResult) string {
	return formatContextResult(ctx, m, projectID, result)
}
