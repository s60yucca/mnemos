package autopilot

import (
	"context"
	"regexp"
	"sort"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

// FindingType identifies the kind of autopilot observation.
type FindingType string

const (
	FindingStaleCompiled          FindingType = "stale_compiled"
	FindingRelationsCreated       FindingType = "relations_created"
	FindingPotentialContradiction FindingType = "potential_contradiction"
)

// Finding is a single observation produced by a detector.
type Finding struct {
	Type     FindingType    `json:"type"`
	Metadata map[string]any `json:"metadata"`
}

// EntityMaps holds the shared extraction results for one project cycle.
type EntityMaps struct {
	Entities  map[string][]string  // memoryID → []entity
	CreatedAt map[string]time.Time // memoryID → CreatedAt
}

// Detector is a single analysis pass over the memory store.
type Detector interface {
	Name() string
	Detect(ctx context.Context, projectID string, maps EntityMaps) ([]Finding, error)
}

// Package-level compiled regexes — compiled once at package init.
var (
	// File paths: must contain at least one "/" separator and a file extension
	reFilePath = regexp.MustCompile(`(?:[\w\-]+/)+[\w\-]+\.\w{1,6}`)
	// Go qualified identifiers: Package.Exported or Type.Method
	reGoIdent = regexp.MustCompile(`\b[A-Z][a-zA-Z0-9]+\.[A-Z][a-zA-Z0-9]+\b`)
	// CLI commands: mnemos subcommand
	reCLICmd = regexp.MustCompile(`\bmnemos\s+[\w][\w\-]*\b`)
	// Config keys: UPPER_SNAKE (5+ chars to avoid noise like TODO/OK/ID) or dotted 3-level notation
	reConfigKey = regexp.MustCompile(`\b[A-Z][A-Z0-9_]{4,}\b|\b\w+\.\w+\.\w+\b`)
)

// ExtractEntities runs all four regex patterns against content and returns a
// deduplicated, sorted slice of entity strings.
func ExtractEntities(content string) []string {
	seen := make(map[string]struct{})

	for _, re := range []*regexp.Regexp{reFilePath, reGoIdent, reCLICmd, reConfigKey} {
		for _, match := range re.FindAllString(content, -1) {
			seen[match] = struct{}{}
		}
	}

	result := make([]string, 0, len(seen))
	for entity := range seen {
		result = append(result, entity)
	}
	sort.Strings(result)
	return result
}

// BuildEntityMaps runs ExtractEntities for each memory and returns EntityMaps.
// Skips memories with type=compiled or category=autopilot.
func BuildEntityMaps(memories []*domain.Memory) EntityMaps {
	maps := EntityMaps{
		Entities:  make(map[string][]string),
		CreatedAt: make(map[string]time.Time),
	}

	for _, m := range memories {
		if m.Type == domain.MemoryTypeCompiled || m.Category == "autopilot" {
			continue
		}
		entities := ExtractEntities(m.Content)
		maps.Entities[m.ID] = entities
		maps.CreatedAt[m.ID] = m.CreatedAt
	}

	return maps
}
