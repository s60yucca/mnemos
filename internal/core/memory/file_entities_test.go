package memory

import (
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

func TestExtractRelatedFiles(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "single file path",
			content: "See auth/session.go for details",
			want:    `["auth/session.go"]`,
		},
		{
			name:    "multiple file paths",
			content: "auth/session.go and internal/core/search.go are related",
			want:    `["auth/session.go","internal/core/search.go"]`,
		},
		{
			name:    "duplicate paths deduplicated",
			content: "auth/session.go is used in auth/session.go",
			want:    `["auth/session.go"]`,
		},
		{
			name:    "no file paths",
			content: "This is just plain text with no paths",
			want:    "",
		},
		{
			name:    "empty string",
			content: "",
			want:    "",
		},
		{
			name:    "root-level file not extracted",
			content: "See main.go for the entry point",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRelatedFiles(tt.content)
			if got != tt.want {
				t.Errorf("extractRelatedFiles(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestParseRelatedFiles(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    []string
	}{
		{
			name:    "valid JSON array",
			encoded: `["auth/session.go","internal/core/search.go"]`,
			want:    []string{"auth/session.go", "internal/core/search.go"},
		},
		{
			name:    "empty string",
			encoded: "",
			want:    nil,
		},
		{
			name:    "malformed JSON",
			encoded: "not-json",
			want:    nil,
		},
		{
			name:    "JSON with empty strings",
			encoded: `["auth/session.go","","internal/core/search.go"]`,
			want:    []string{"auth/session.go", "internal/core/search.go"},
		},
		{
			name:    "all empty strings",
			encoded: `["",""]`,
			want:    nil,
		},
		{
			name:    "JSON object (non-array)",
			encoded: `{"key":"value"}`,
			want:    nil,
		},
		{
			name:    "JSON number (non-array)",
			encoded: `42`,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRelatedFiles(tt.encoded)
			if len(got) != len(tt.want) {
				t.Errorf("parseRelatedFiles(%q) = %v, want %v", tt.encoded, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseRelatedFiles(%q)[%d] = %q, want %q", tt.encoded, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFileOverlap(t *testing.T) {
	tests := []struct {
		name string
		mf   string
		of   string
		want bool
	}{
		{"exact match", "auth/session.go", "auth/session.go", true},
		{"dot-slash prefix on of", "auth/session.go", "./auth/session.go", true},
		{"dot-slash prefix on mf", "./auth/session.go", "auth/session.go", true},
		{"both dot-slash prefixed", "./auth/session.go", "./auth/session.go", true},
		{"case mismatch no match", "auth/Session.go", "auth/session.go", false},
		{"different paths", "auth/session.go", "internal/core/search.go", false},
		{"empty strings", "", "", true},
		{"one empty", "auth/session.go", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileOverlap(tt.mf, tt.of)
			if got != tt.want {
				t.Errorf("fileOverlap(%q, %q) = %v, want %v", tt.mf, tt.of, got, tt.want)
			}
		})
	}
}

func TestFileBoost(t *testing.T) {
	tests := []struct {
		name      string
		memFiles  []string
		openFiles []string
		want      float64
	}{
		{"overlapping", []string{"auth/session.go"}, []string{"auth/session.go"}, FileBoostDefault},
		{"non-overlapping", []string{"auth/session.go"}, []string{"internal/core/search.go"}, 0.0},
		{"multiple overlaps still one boost", []string{"auth/session.go", "internal/core/search.go"}, []string{"auth/session.go", "internal/core/search.go"}, FileBoostDefault},
		{"nil memFiles", nil, []string{"auth/session.go"}, 0.0},
		{"nil openFiles", []string{"auth/session.go"}, nil, 0.0},
		{"empty memFiles", []string{}, []string{"auth/session.go"}, 0.0},
		{"empty openFiles", []string{"auth/session.go"}, []string{}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileBoost(tt.memFiles, tt.openFiles)
			if got != tt.want {
				t.Errorf("fileBoost(%v, %v) = %f, want %f", tt.memFiles, tt.openFiles, got, tt.want)
			}
		})
	}
}

func TestApplyFileBoost(t *testing.T) {
	makeMemory := func(score float64, encoded string) *domain.Memory {
		m := &domain.Memory{RelevanceScore: score}
		if encoded != "" {
			m.Metadata = map[string]string{RelatedFilesKey: encoded}
		}
		return m
	}

	t.Run("nil openFiles is no-op", func(t *testing.T) {
		mem := makeMemory(0.5, `["auth/session.go"]`)
		ApplyFileBoost([]*domain.Memory{mem}, nil, FileBoostDefault)
		if mem.RelevanceScore != 0.5 {
			t.Errorf("score changed with nil openFiles: got %f", mem.RelevanceScore)
		}
	})

	t.Run("empty openFiles is no-op", func(t *testing.T) {
		mem := makeMemory(0.5, `["auth/session.go"]`)
		ApplyFileBoost([]*domain.Memory{mem}, []string{}, FileBoostDefault)
		if mem.RelevanceScore != 0.5 {
			t.Errorf("score changed with empty openFiles: got %f", mem.RelevanceScore)
		}
	})

	t.Run("partial overlap boosts only matching memory", func(t *testing.T) {
		mem1 := makeMemory(0.5, `["auth/session.go"]`)
		mem2 := makeMemory(0.5, `["internal/core/search.go"]`)
		openFiles := []string{"auth/session.go"}
		ApplyFileBoost([]*domain.Memory{mem1, mem2}, openFiles, FileBoostDefault)
		if mem1.RelevanceScore != 0.5+FileBoostDefault {
			t.Errorf("mem1 score = %f, want %f", mem1.RelevanceScore, 0.5+FileBoostDefault)
		}
		if mem2.RelevanceScore != 0.5 {
			t.Errorf("mem2 score changed: got %f, want 0.5", mem2.RelevanceScore)
		}
	})

	t.Run("no related_files key is no-op", func(t *testing.T) {
		mem := makeMemory(0.5, "")
		ApplyFileBoost([]*domain.Memory{mem}, []string{"auth/session.go"}, FileBoostDefault)
		if mem.RelevanceScore != 0.5 {
			t.Errorf("score changed for memory with no related_files: got %f", mem.RelevanceScore)
		}
	})

	t.Run("score can exceed 1.0", func(t *testing.T) {
		mem := makeMemory(0.9, `["auth/session.go"]`)
		ApplyFileBoost([]*domain.Memory{mem}, []string{"auth/session.go"}, FileBoostDefault)
		expected := 0.9 + FileBoostDefault
		if mem.RelevanceScore != expected {
			t.Errorf("score = %f, want %f", mem.RelevanceScore, expected)
		}
	})
}
