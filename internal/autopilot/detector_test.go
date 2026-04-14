package autopilot

import (
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

// --- ExtractEntities tests ---

func TestExtractEntities_FilePath(t *testing.T) {
	entities := ExtractEntities("see internal/storage/sqlite/store.go")
	if !contains(entities, "internal/storage/sqlite/store.go") {
		t.Errorf("expected 'internal/storage/sqlite/store.go' in %v", entities)
	}
}

func TestExtractEntities_GoIdent(t *testing.T) {
	entities := ExtractEntities("SQLiteStore.GetByID panics")
	if !contains(entities, "SQLiteStore.GetByID") {
		t.Errorf("expected 'SQLiteStore.GetByID' in %v", entities)
	}
}

func TestExtractEntities_CLICmd(t *testing.T) {
	entities := ExtractEntities("run mnemos serve first")
	if !contains(entities, "mnemos serve") {
		t.Errorf("expected 'mnemos serve' in %v", entities)
	}
}

func TestExtractEntities_ConfigKey(t *testing.T) {
	entities := ExtractEntities("set MNEMOS_DB_PATH")
	if !contains(entities, "MNEMOS_DB_PATH") {
		t.Errorf("expected 'MNEMOS_DB_PATH' in %v", entities)
	}
}

// TestExtractEntities_ConfigKeyNoiseFiltered verifies that short UPPER tokens
// like TODO, OK, ID, API, URL are not extracted (minimum 5 chars enforced).
func TestExtractEntities_ConfigKeyNoiseFiltered(t *testing.T) {
	noisy := []string{"TODO", "OK", "ID", "API", "URL", "GET", "PUT", "EOF"}
	for _, word := range noisy {
		entities := ExtractEntities("the " + word + " value")
		for _, e := range entities {
			if e == word {
				t.Errorf("short token %q should not be extracted as a config key entity", word)
			}
		}
	}
}

func TestExtractEntities_NoMatch(t *testing.T) {
	entities := ExtractEntities("the project is good")
	if len(entities) != 0 {
		t.Errorf("expected empty slice, got %v", entities)
	}
}

func TestExtractEntities_Sorted(t *testing.T) {
	entities := ExtractEntities("see internal/foo/bar.go and internal/baz/qux.go")
	for i := 1; i < len(entities); i++ {
		if entities[i] < entities[i-1] {
			t.Errorf("result not sorted: %v", entities)
		}
	}
}

func TestExtractEntities_Deduplicated(t *testing.T) {
	entities := ExtractEntities("internal/foo/bar.go and internal/foo/bar.go again")
	count := 0
	for _, e := range entities {
		if e == "internal/foo/bar.go" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of 'internal/foo/bar.go', got %d in %v", count, entities)
	}
}

// --- BuildEntityMaps tests ---

func TestBuildEntityMaps_SkipsCompiled(t *testing.T) {
	compiled := &domain.Memory{
		ID:       "compiled-1",
		Type:     domain.MemoryTypeCompiled,
		Category: "code",
		Content:  "see internal/storage/sqlite/store.go",
	}
	maps := BuildEntityMaps([]*domain.Memory{compiled})
	if _, ok := maps.Entities["compiled-1"]; ok {
		t.Error("compiled memory should not appear in Entities map")
	}
}

func TestBuildEntityMaps_SkipsAutopilotCategory(t *testing.T) {
	report := &domain.Memory{
		ID:       "report-1",
		Type:     domain.MemoryTypeSemantic,
		Category: "autopilot",
		Content:  "## Autopilot Report internal/foo/bar.go",
	}
	maps := BuildEntityMaps([]*domain.Memory{report})
	if _, ok := maps.Entities["report-1"]; ok {
		t.Error("autopilot category memory should not appear in Entities map")
	}
}

func TestBuildEntityMaps_PopulatesCreatedAt(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	mem := &domain.Memory{
		ID:        "mem-1",
		Type:      domain.MemoryTypeSemantic,
		Category:  "code",
		Content:   "see internal/storage/sqlite/store.go",
		CreatedAt: ts,
	}
	maps := BuildEntityMaps([]*domain.Memory{mem})
	got, ok := maps.CreatedAt["mem-1"]
	if !ok {
		t.Fatal("expected mem-1 in CreatedAt map")
	}
	if !got.Equal(ts) {
		t.Errorf("expected %v, got %v", ts, got)
	}
}

func TestBuildEntityMaps_PopulatesEntities(t *testing.T) {
	mem := &domain.Memory{
		ID:       "mem-2",
		Type:     domain.MemoryTypeSemantic,
		Category: "code",
		Content:  "see internal/storage/sqlite/store.go",
	}
	maps := BuildEntityMaps([]*domain.Memory{mem})
	entities, ok := maps.Entities["mem-2"]
	if !ok {
		t.Fatal("expected mem-2 in Entities map")
	}
	if !contains(entities, "internal/storage/sqlite/store.go") {
		t.Errorf("expected file path entity in %v", entities)
	}
}

func TestBuildEntityMaps_EmptyContentMemory(t *testing.T) {
	mem := &domain.Memory{
		ID:       "mem-3",
		Type:     domain.MemoryTypeSemantic,
		Category: "code",
		Content:  "the project is good",
	}
	maps := BuildEntityMaps([]*domain.Memory{mem})
	// Should still be in the map (just with empty entities)
	if _, ok := maps.CreatedAt["mem-3"]; !ok {
		t.Error("expected mem-3 in CreatedAt map even with no entities")
	}
}

// --- helpers ---

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
