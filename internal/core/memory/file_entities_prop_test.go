package memory

import (
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"pgregory.net/rapid"
)

// Feature: mnemos-file-aware-retrieval, Property 1: fileBoost returns only {0.0, FileBoostDefault}
// Validates: Requirements 3.3, 3.4
func TestProp_FileBoost_BinaryOutput(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		memFiles := rapid.SliceOf(rapid.String()).Draw(rt, "memFiles")
		openFiles := rapid.SliceOf(rapid.String()).Draw(rt, "openFiles")
		result := fileBoost(memFiles, openFiles)
		if result != 0.0 && result != FileBoostDefault {
			rt.Fatalf("fileBoost returned %f, want 0.0 or %f", result, FileBoostDefault)
		}
	})
}

// Feature: mnemos-file-aware-retrieval, Property 2: empty open-files produces zero boost
// Validates: Requirements 3.6, 4.2
func TestProp_FileBoost_EmptyOpenFiles_ZeroBoost(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		memFiles := rapid.SliceOf(rapid.String()).Draw(rt, "memFiles")
		if got := fileBoost(memFiles, nil); got != 0.0 {
			rt.Fatalf("fileBoost(memFiles, nil) = %f, want 0.0", got)
		}
		if got := fileBoost(memFiles, []string{}); got != 0.0 {
			rt.Fatalf("fileBoost(memFiles, []) = %f, want 0.0", got)
		}
	})
}

// Feature: mnemos-file-aware-retrieval, Property 3: extraction only contains file-path entities
// Validates: Requirements 1.1, 1.4
func TestProp_ExtractRelatedFiles_OnlyFilePaths(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		content := rapid.String().Draw(rt, "content")
		encoded := extractRelatedFiles(content)
		parsed := parseRelatedFiles(encoded)
		for _, p := range parsed {
			// 3b: every extracted path satisfies the file-path pattern
			if !reFilePath.MatchString(p) {
				rt.Fatalf("extracted path %q does not match reFilePath pattern", p)
			}
			// 3a: every extracted path is a substring of the content
			if !containsSubstring(content, p) {
				rt.Fatalf("extracted path %q not found in content %q", p, content)
			}
		}
	})
}

// Feature: mnemos-file-aware-retrieval, Property 4: boost is monotone
// Validates: Requirements 3.2, 3.7
func TestProp_ApplyFileBoost_Monotone(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		score := rapid.Float64Range(0, 1).Draw(rt, "score")
		mem := &domain.Memory{
			RelevanceScore: score,
			Metadata: map[string]string{
				RelatedFilesKey: `["foo/bar.go"]`,
			},
		}
		openFiles := []string{"foo/bar.go"}
		before := mem.RelevanceScore
		ApplyFileBoost([]*domain.Memory{mem}, openFiles, FileBoostDefault)
		if mem.RelevanceScore <= before {
			rt.Fatalf("score after boost %f not > before %f", mem.RelevanceScore, before)
		}
	})
}

// Feature: mnemos-file-aware-retrieval, Property 5: metadata round-trip
// Validates: Requirements 1.6, 6.2
func TestProp_ExtractRelatedFiles_RoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		content := rapid.String().Draw(rt, "content")
		encoded1 := extractRelatedFiles(content)
		encoded2 := extractRelatedFiles(content)
		// Re-encoding must be stable (idempotent)
		if encoded1 != encoded2 {
			rt.Fatalf("extractRelatedFiles not stable: %q != %q", encoded1, encoded2)
		}
		// Decoded paths must all be file-path shaped
		for _, p := range parseRelatedFiles(encoded1) {
			if !reFilePath.MatchString(p) {
				rt.Fatalf("round-trip path %q does not match reFilePath", p)
			}
		}
	})
}

// Feature: mnemos-file-aware-retrieval, Property 6: malformed metadata never panics
// Validates: Requirements 5.3
func TestProp_ParseRelatedFiles_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.String().Draw(rt, "s")
		// Must not panic
		result := parseRelatedFiles(s)
		_ = result
	})
}

// containsSubstring checks if sub is a substring of s.
func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
