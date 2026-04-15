package memory

import (
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestSplitSentences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty input returns empty slice",
			input: "",
			want:  []string{},
		},
		{
			name:  "single sentence no terminator",
			input: "Hello world",
			want:  []string{"Hello world"},
		},
		{
			name:  "multi-sentence prose split on period-space",
			input: "The dog barked. The cat ran. The bird flew.",
			want:  []string{"The dog barked.", "The cat ran.", "The bird flew."},
		},
		{
			name:  "split on exclamation mark",
			input: "Watch out! Something happened.",
			want:  []string{"Watch out!", "Something happened."},
		},
		{
			name:  "split on question mark",
			input: "What is this? It is a test.",
			want:  []string{"What is this?", "It is a test."},
		},
		{
			name:  "split on double newline",
			input: "First paragraph.\n\nSecond paragraph.",
			want:  []string{"First paragraph.", "Second paragraph."},
		},
		{
			name:  "fenced code block stripped, surrounding text preserved",
			input: "Before code.\n```\nsome code here\n```\nAfter code.",
			want:  []string{"Before code.", "After code."},
		},
		{
			name:  "content that is only a fenced code block returns empty slice",
			input: "```\nonly code\n```",
			want:  []string{},
		},
		{
			name:  "inline backticks preserved in output",
			input: "Use `SessionStore` carefully.",
			want:  []string{"Use `SessionStore` carefully."},
		},
		{
			name:  "adjacent inline backticks preserved",
			input: "Call `foo` and `bar` together.",
			want:  []string{"Call `foo` and `bar` together."},
		},
		{
			name:  "no terminators returns single sentence",
			input: "This has no sentence terminators at all",
			want:  []string{"This has no sentence terminators at all"},
		},
		{
			name:  "trailing newlines trimmed",
			input: "Hello world\n\n",
			want:  []string{"Hello world"},
		},
		{
			name:  "numbered list not split on digit-preceded dot",
			input: "Steps:\n1. First\n2. Second",
			want:  []string{"Steps:\n1. First\n2. Second"},
		},
		{
			name:  "decimal number not split",
			input: "threshold of 0.7 works",
			want:  []string{"threshold of 0.7 works"},
		},
		{
			name:  "decimal number at end of sentence not split (digit precedes dot)",
			input: "The threshold is 0.7. Use it carefully.",
			want:  []string{"The threshold is 0.7. Use it carefully."},
		},
		{
			name:  "mixed prose and bullets",
			input: "Overview of steps. - First do this. - Then do that.",
			want:  []string{"Overview of steps.", "- First do this.", "- Then do that."},
		},
		{
			name:  "fenced code block with surrounding prose",
			input: "Here is an example.\n```\nfunc foo() {}\n```\nThat was the example.",
			want:  []string{"Here is an example.", "That was the example."},
		},
		{
			name:  "leading and trailing whitespace trimmed from sentences",
			input: "  Hello world.   Next sentence.  ",
			want:  []string{"Hello world.", "Next sentence."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSentences(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInformationScore(t *testing.T) {
	t.Run("isFirst bonus fires", func(t *testing.T) {
		s := "Hello world test sentence"
		withFirst := informationScore(s, true)
		withoutFirst := informationScore(s, false)
		assert.Greater(t, withFirst, withoutFirst)
	})

	t.Run("CamelCase bonus fires", func(t *testing.T) {
		base := "the function handles authentication logic"
		withCamel := informationScore("the SessionStore handles authentication logic", false)
		baseScore := informationScore(base, false)
		assert.Greater(t, withCamel, baseScore)
	})

	t.Run("file-path bonus fires", func(t *testing.T) {
		base := "the module handles storage operations"
		withPath := informationScore("the internal/storage/store.go handles storage operations", false)
		baseScore := informationScore(base, false)
		assert.Greater(t, withPath, baseScore)
	})

	t.Run("causal word bonus fires", func(t *testing.T) {
		base := "the system failed during startup"
		withCausal := informationScore("the system failed because of startup", false)
		baseScore := informationScore(base, false)
		assert.Greater(t, withCausal, baseScore)
	})

	t.Run("config-key bonus fires", func(t *testing.T) {
		base := "the token is used for authentication"
		withConfig := informationScore("the JWT_SECRET is used for authentication", false)
		baseScore := informationScore(base, false)
		assert.Greater(t, withConfig, baseScore)
	})

	t.Run("short sentence penalty fires", func(t *testing.T) {
		short := informationScore("hi there", false)               // 2 words
		longer := informationScore("hi there friend today", false) // 4 words
		assert.Less(t, short, longer)
	})

	t.Run("stop word penalty fires", func(t *testing.T) {
		// High stop-word ratio sentence
		highStop := informationScore("the and is a of to in", false)
		// Low stop-word ratio sentence
		lowStop := informationScore("SessionStore initializes database connection pool", false)
		assert.Less(t, highStop, lowStop)
	})

	t.Run("clamp prevents negative return", func(t *testing.T) {
		// Very short sentence with all stop words should not go negative
		score := informationScore("a", false)
		assert.GreaterOrEqual(t, score, 0.0)
	})

	t.Run("deterministic same input returns same output", func(t *testing.T) {
		s := "SessionStore initializes the database connection pool"
		first := informationScore(s, true)
		second := informationScore(s, true)
		assert.Equal(t, first, second)
	})

	t.Run("unique token deduplication for CamelCase", func(t *testing.T) {
		// "SessionStore calls SessionStore" — SessionStore appears twice, should only give +0.2 once
		deduped := informationScore("SessionStore calls SessionStore", false)
		// "SessionStore calls OtherStore" — two unique CamelCase tokens, should give +0.4
		twoTokens := informationScore("SessionStore calls OtherStore", false)
		assert.Less(t, deduped, twoTokens)
	})
}

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		memType   domain.MemoryType
		maxWords  int
		wantEmpty bool
		check     func(t *testing.T, got string)
	}{
		{
			name:      "empty input returns empty string",
			content:   "",
			memType:   domain.MemoryTypeShortTerm,
			maxWords:  30,
			wantEmpty: true,
		},
		{
			name:      "ten words or fewer returns empty string",
			content:   "Short content here with only nine words total",
			memType:   domain.MemoryTypeShortTerm,
			maxWords:  30,
			wantEmpty: true,
		},
		{
			name:     "single sentence longer than 10 words returns that sentence",
			content:  "This is a single sentence that has more than ten words in it.",
			memType:  domain.MemoryTypeShortTerm,
			maxWords: 30,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				assert.Contains(t, got, "single sentence")
			},
		},
		{
			name:     "multi-sentence generic returns up to 3 sentences in original order",
			content:  "First sentence has important information. Second sentence adds more context here. Third sentence concludes the thought nicely. Fourth sentence is extra and may be dropped.",
			memType:  domain.MemoryTypeShortTerm,
			maxWords: 50,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				words := strings.Fields(got)
				assert.LessOrEqual(t, len(words), 50)
				// result should be in document order (first before third if both selected)
				firstIdx := strings.Index(got, "First")
				thirdIdx := strings.Index(got, "Third")
				if firstIdx != -1 && thirdIdx != -1 {
					assert.Less(t, firstIdx, thirdIdx)
				}
			},
		},
		{
			name:     "skill memory with numbered steps prefers step sentences",
			content:  "This is an introduction to the process. Follow these steps carefully.\n1. Install the dependencies\n2. Configure the environment\n3. Run the application. This is a conclusion.",
			memType:  domain.MemoryTypeSkill,
			maxWords: 50,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				// The step sentence should be included due to +0.4 modifier
				assert.Contains(t, got, "1.")
			},
		},
		{
			name:     "skill memory with backtick tokens prefers backtick sentences",
			content:  "This is a plain introduction without any code references at all. Use `SessionStore` to manage sessions efficiently. Another plain sentence without technical content here. Yet another plain sentence to pad the content.",
			memType:  domain.MemoryTypeSkill,
			maxWords: 50,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				// The backtick sentence should be preferred due to +0.3 modifier
				assert.Contains(t, got, "`SessionStore`")
			},
		},
		{
			name:     "compiled memory prefers first and last sentences",
			content:  "This is the thesis statement of the compiled article. Middle sentence one adds supporting detail. Middle sentence two adds more supporting detail. This is the conclusion of the compiled article.",
			memType:  domain.MemoryTypeCompiled,
			maxWords: 50,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				// First and last should be preferred due to +0.3 each
				assert.Contains(t, got, "thesis statement")
				assert.Contains(t, got, "conclusion")
			},
		},
		{
			name:     "episodic memory prefers last sentence",
			content:  "We started the investigation into the database issue. We checked the connection pool settings carefully. We found the root cause and applied the fix successfully.",
			memType:  domain.MemoryTypeEpisodic,
			maxWords: 50,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				// Last sentence gets +0.2 modifier
				assert.Contains(t, got, "root cause")
			},
		},
		{
			name:     "maxWords truncation limits result word count",
			content:  "This is a fairly long sentence with many words in it. Another sentence adds even more words to the total count. A third sentence continues to add words beyond the limit.",
			memType:  domain.MemoryTypeShortTerm,
			maxWords: 8,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				words := strings.Fields(got)
				assert.LessOrEqual(t, len(words), 8)
			},
		},
		{
			name:     "causal word bonus fires and prefers causal sentence",
			content:  "The system encountered an error during startup. The authentication module failed because of an expired certificate. The logs showed no other issues.",
			memType:  domain.MemoryTypeShortTerm,
			maxWords: 50,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				// Sentence with "because" gets causal bonus
				assert.Contains(t, got, "because")
			},
		},
		{
			name:     "code-heavy content strips fenced block and extracts surrounding prose",
			content:  "Here is how to configure the server.\n```\nserver:\n  port: 8080\n  host: localhost\n```\nThe configuration above sets the default port and host values.",
			memType:  domain.MemoryTypeShortTerm,
			maxWords: 30,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				// Code block content should not appear
				assert.NotContains(t, got, "port: 8080")
				assert.NotContains(t, got, "host: localhost")
				// Surrounding prose should be present
				assert.True(t,
					strings.Contains(got, "configure the server") ||
						strings.Contains(got, "configuration above"),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSummary(tt.content, tt.memType, tt.maxWords)
			if tt.wantEmpty {
				assert.Empty(t, got)
				return
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
