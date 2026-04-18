package memory

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

const (
	// FileBoostDefault is the additive score increment applied when a memory's
	// file entities overlap with the open files list.
	FileBoostDefault = 0.3

	// RelatedFilesKey is the Metadata map key for the JSON-encoded file path array.
	RelatedFilesKey = "related_files"
)

// reFilePath mirrors the pattern in internal/autopilot/detector.go.
// Importing autopilot from core/memory would create an import cycle
// (autopilot/backfill.go imports core/memory). The regex is kept in sync
// manually; it must contain at least one "/" separator and a file extension.
var reFilePath = regexp.MustCompile(`(?:[\w\-]+/)+[\w\-]+\.\w{1,6}`)

// extractRelatedFiles calls autopilot.ExtractEntities on content, filters to
// file-path entities using autopilot.IsFilePath, deduplicates, and returns them
// as a JSON-encoded string array. Returns "" when no file entities are found.
// Panics from ExtractEntities are recovered; "" is returned on panic.
//
// File paths must contain at least one directory separator to be extracted.
// Root-level files (main.go, Dockerfile) are not matched by the reFilePath
// pattern and will never receive a file-aware boost.
func extractRelatedFiles(content string) (encoded string) {
	defer func() {
		if r := recover(); r != nil {
			encoded = ""
		}
	}()

	matches := reFilePath.FindAllString(content, -1)
	seen := make(map[string]struct{})
	var filePaths []string
	for _, e := range matches {
		if _, ok := seen[e]; !ok {
			seen[e] = struct{}{}
			filePaths = append(filePaths, e)
		}
	}
	if len(filePaths) == 0 {
		return ""
	}
	b, err := json.Marshal(filePaths)
	if err != nil {
		return ""
	}
	return string(b)
}

// parseRelatedFiles decodes the JSON array stored in Metadata["related_files"].
// Returns nil for empty, missing, or malformed values. Empty strings within
// the array are silently skipped.
func parseRelatedFiles(encoded string) []string {
	if encoded == "" {
		return nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(encoded), &paths); err != nil {
		return nil
	}
	result := paths[:0]
	for _, p := range paths {
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// fileOverlap returns true if mf and of are byte-equal after stripping any
// leading "./" prefix from each. No other normalisation is applied.
func fileOverlap(mf, of string) bool {
	return strings.TrimPrefix(mf, "./") == strings.TrimPrefix(of, "./")
}

// fileBoost returns FileBoostDefault if any path in memFiles overlaps with any
// path in openFiles (binary: applied at most once). Returns 0.0 otherwise.
func fileBoost(memFiles, openFiles []string) float64 {
	if len(memFiles) == 0 || len(openFiles) == 0 {
		return 0.0
	}
	for _, mf := range memFiles {
		for _, of := range openFiles {
			if fileOverlap(mf, of) {
				return FileBoostDefault
			}
		}
	}
	return 0.0
}

// ApplyFileBoost mutates each memory's RelevanceScore in-place by adding
// fileBoost(parseRelatedFiles(mem.Metadata[RelatedFilesKey]), openFiles).
// Exported because it is called from package search (engine.go).
// No-ops when openFiles is nil or empty.
//
// Note: RelevanceScore may exceed 1.0 after this call. This is intentional;
// downstream components (DiversityFilter, PackWithBudget) handle unbounded scores.
func ApplyFileBoost(memories []*domain.Memory, openFiles []string, boostVal float64) {
	if len(openFiles) == 0 {
		return
	}
	for _, mem := range memories {
		var encoded string
		if mem.Metadata != nil {
			encoded = mem.Metadata[RelatedFilesKey]
		}
		memFiles := parseRelatedFiles(encoded)
		boost := fileBoost(memFiles, openFiles)
		if boost > 0 {
			mem.RelevanceScore += boostVal
		}
	}
}
