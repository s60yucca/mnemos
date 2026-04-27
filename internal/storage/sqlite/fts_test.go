package sqlite

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

func TestSanitizeFTSQuery_SingleToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple word",
			input: "hello",
			want:  `"hello"`,
		},
		{
			name:  "word with spaces",
			input: "  hello  ",
			want:  `"hello"`,
		},
		{
			name:  "word with special chars",
			input: "hello*world",
			want:  `"hello" OR "world"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFTSQuery(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFTSQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeFTSQuery_MultiToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "two words",
			input: "hello world",
			want:  `"hello" OR "world"`,
		},
		{
			name:  "three words",
			input: "foo bar baz",
			want:  `"foo" OR "bar" OR "baz"`,
		},
		{
			name:  "words with extra spaces",
			input: "hello   world   test",
			want:  `"hello" OR "world" OR "test"`,
		},
		{
			name:  "HMS query from bug report",
			input: "HMS current project context deployment payment",
			want:  `"HMS" OR "current" OR "project" OR "context" OR "deployment" OR "payment"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFTSQuery(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFTSQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeFTSQuery_EmptyQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  `""`,
		},
		{
			name:  "only spaces",
			input: "   ",
			want:  `""`,
		},
		{
			name:  "only special chars",
			input: "***---:::///",
			want:  `""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFTSQuery(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFTSQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeFTSQuery_LongQuery(t *testing.T) {
	// Generate a query with 50 tokens
	tokens := make([]string, 50)
	for i := 0; i < 50; i++ {
		tokens[i] = "word"
	}
	input := strings.Join(tokens, " ")

	got := sanitizeFTSQuery(input)

	// Should truncate to maxQueryTokens (30)
	gotTokens := strings.Split(got, " OR ")
	if len(gotTokens) != maxQueryTokens {
		t.Errorf("sanitizeFTSQuery with 50 tokens returned %d tokens, want %d", len(gotTokens), maxQueryTokens)
	}

	// Each token should be quoted
	for i, tok := range gotTokens {
		if tok != `"word"` {
			t.Errorf("token %d = %q, want %q", i, tok, `"word"`)
		}
	}
}

func TestSanitizeFTSQuery_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "parentheses",
			input: "(hello world)",
			want:  `"hello" OR "world"`,
		},
		{
			name:  "quotes",
			input: `"hello world"`,
			want:  `"hello" OR "world"`,
		},
		{
			name:  "asterisk",
			input: "hello*",
			want:  `"hello"`,
		},
		{
			name:  "colon",
			input: "key:value",
			want:  `"key" OR "value"`,
		},
		{
			name:  "caret",
			input: "^start",
			want:  `"start"`,
		},
		{
			name:  "dash",
			input: "hello-world",
			want:  `"hello" OR "world"`,
		},
		{
			name:  "slash",
			input: "path/to/file",
			want:  `"path" OR "to" OR "file"`,
		},
		{
			name:  "backslash",
			input: `path\to\file`,
			want:  `"path" OR "to" OR "file"`,
		},
		{
			name:  "mixed special chars",
			input: `(hello*) "world" -test:value`,
			want:  `"hello" OR "world" OR "test" OR "value"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFTSQuery(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFTSQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFTSSearch_MultiKeywordReturnsResults(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Insert test memories
	now := time.Now().UnixNano()
	memories := []*domain.Memory{
		{
			ID:          "mem1",
			Content:     "HMS deployment configuration for production environment",
			Type:        domain.MemoryTypeEpisodic,
			ProjectID:   "hms",
			Status:      domain.MemoryStatusActive,
			ContentHash: "hash1",
			CreatedAt:   util.UnixNanoToTime(now),
			UpdatedAt:   util.UnixNanoToTime(now),
		},
		{
			ID:          "mem2",
			Content:     "Payment collection workflow and ledger updates",
			Type:        domain.MemoryTypeEpisodic,
			ProjectID:   "hms",
			Status:      domain.MemoryStatusActive,
			ContentHash: "hash2",
			CreatedAt:   util.UnixNanoToTime(now),
			UpdatedAt:   util.UnixNanoToTime(now),
		},
		{
			ID:          "mem3",
			Content:     "Shift close procedures and deposit reconciliation",
			Type:        domain.MemoryTypeEpisodic,
			ProjectID:   "hms",
			Status:      domain.MemoryStatusActive,
			ContentHash: "hash3",
			CreatedAt:   util.UnixNanoToTime(now),
			UpdatedAt:   util.UnixNanoToTime(now),
		},
		{
			ID:          "mem4",
			Content:     "Unrelated content about cats and dogs",
			Type:        domain.MemoryTypeEpisodic,
			ProjectID:   "other",
			Status:      domain.MemoryStatusActive,
			ContentHash: "hash4",
			CreatedAt:   util.UnixNanoToTime(now),
			UpdatedAt:   util.UnixNanoToTime(now),
		},
	}

	for _, m := range memories {
		_, err := db.Exec(`
			INSERT INTO memories (id, content, type, project_id, status, content_hash, created_at, updated_at, last_accessed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, m.Content, m.Type, m.ProjectID, m.Status, m.ContentHash, util.TimeToUnixNano(m.CreatedAt), util.TimeToUnixNano(m.UpdatedAt), util.TimeToUnixNano(m.CreatedAt))
		if err != nil {
			t.Fatalf("Insert memory %s: %v", m.ID, err)
		}
	}

	searcher := NewFTSSearcher(db)

	// Test multi-keyword query (should match any keyword with OR logic)
	// Query contains keywords that match different memories:
	// - "HMS" and "deployment" match mem1
	// - "payment" and "ledger" match mem2
	// - "shift" and "deposit" match mem3
	results, err := searcher.Search(context.Background(), storage.TextSearchQuery{
		Query:     "HMS payment shift",
		ProjectID: "hms",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// Should return all 3 HMS memories (each matches at least one keyword)
	if len(results) != 3 {
		t.Errorf("Search returned %d results, want 3", len(results))
		for i, r := range results {
			t.Logf("Result %d: %s - %s", i, r.Memory.ID, r.Memory.Content)
		}
	}

	// Verify all results are from HMS project
	for _, r := range results {
		if r.Memory.ProjectID != "hms" {
			t.Errorf("Result %s has project_id %q, want %q", r.Memory.ID, r.Memory.ProjectID, "hms")
		}
	}
}

func TestFTSSearch_RankingPrefersMoreMatches(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Insert test memories with varying keyword matches
	now := time.Now().UnixNano()
	memories := []*domain.Memory{
		{
			ID:          "mem1",
			Content:     "apple banana cherry", // matches 3 keywords
			Type:        domain.MemoryTypeEpisodic,
			ProjectID:   "test",
			Status:      domain.MemoryStatusActive,
			ContentHash: "hash1",
			CreatedAt:   util.UnixNanoToTime(now),
			UpdatedAt:   util.UnixNanoToTime(now),
		},
		{
			ID:          "mem2",
			Content:     "apple banana", // matches 2 keywords
			Type:        domain.MemoryTypeEpisodic,
			ProjectID:   "test",
			Status:      domain.MemoryStatusActive,
			ContentHash: "hash2",
			CreatedAt:   util.UnixNanoToTime(now),
			UpdatedAt:   util.UnixNanoToTime(now),
		},
		{
			ID:          "mem3",
			Content:     "apple", // matches 1 keyword
			Type:        domain.MemoryTypeEpisodic,
			ProjectID:   "test",
			Status:      domain.MemoryStatusActive,
			ContentHash: "hash3",
			CreatedAt:   util.UnixNanoToTime(now),
			UpdatedAt:   util.UnixNanoToTime(now),
		},
	}

	for _, m := range memories {
		_, err := db.Exec(`
			INSERT INTO memories (id, content, type, project_id, status, content_hash, created_at, updated_at, last_accessed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, m.Content, m.Type, m.ProjectID, m.Status, m.ContentHash, util.TimeToUnixNano(m.CreatedAt), util.TimeToUnixNano(m.UpdatedAt), util.TimeToUnixNano(m.CreatedAt))
		if err != nil {
			t.Fatalf("Insert memory %s: %v", m.ID, err)
		}
	}

	searcher := NewFTSSearcher(db)

	results, err := searcher.Search(context.Background(), storage.TextSearchQuery{
		Query:     "apple banana cherry",
		ProjectID: "test",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Search returned %d results, want 3", len(results))
	}

	// BM25 ranks by relevance (lower score = more relevant in SQLite FTS5)
	// Memory with more keyword matches should rank higher (lower score)
	if results[0].Memory.ID != "mem1" {
		t.Errorf("Top result is %s, want mem1 (matches all 3 keywords)", results[0].Memory.ID)
	}

	// Verify scores are ordered (lower = better)
	for i := 1; i < len(results); i++ {
		if results[i].TextScore < results[i-1].TextScore {
			t.Errorf("Result %d score (%f) is lower than result %d score (%f), ranking is incorrect",
				i, results[i].TextScore, i-1, results[i-1].TextScore)
		}
	}
}
