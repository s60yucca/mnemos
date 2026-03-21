package hook_test

import (
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/hook"
	"pgregory.net/rapid"
)

// Feature: mnemos-autopilot, Property 12: For any prompt text from the known generic set, IsGenericPrompt must return true.
// For any prompt text that is clearly specific (contains non-generic words), IsGenericPrompt must return false.
func TestProp_IsGenericPrompt(t *testing.T) {
	// The exhaustive generic set as defined in heuristics.go
	genericPrompts := []string{
		"continue", "ok", "okay", "yes", "no", "sure", "thanks",
		"thank you", "go ahead", "proceed", "next", "keep going",
		"looks good", "lgtm", "do it", "go on", "right", "yep",
		"yeah", "nah", "nope", "fine", "great", "perfect",
		"sounds good", "makes sense", "got it", "understood",
	}

	// Clearly specific prompts that contain non-generic domain words
	specificPrompts := []string{
		"fix the auth bug",
		"implement jwt authentication",
		"refactor database connection pooling",
		"add unit tests for middleware",
		"deploy to production server",
		"optimize query performance",
		"configure redis caching",
		"debug memory leak goroutine",
	}

	t.Run("generic prompts return true", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			// Draw a generic prompt, optionally with trailing punctuation and mixed case
			base := rapid.SampledFrom(genericPrompts).Draw(rt, "generic_prompt")
			suffix := rapid.SampledFrom([]string{"", ".", "!", "?"}).Draw(rt, "suffix")
			// Mix case randomly
			useUpper := rapid.Bool().Draw(rt, "use_upper")
			input := base + suffix
			if useUpper {
				input = strings.ToUpper(string(input[0])) + input[1:]
			}

			if !hook.IsGenericPrompt(input) {
				rt.Fatalf("IsGenericPrompt(%q) = false, want true (base: %q)", input, base)
			}
		})
	})

	t.Run("specific prompts return false", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			specific := rapid.SampledFrom(specificPrompts).Draw(rt, "specific_prompt")

			if hook.IsGenericPrompt(specific) {
				rt.Fatalf("IsGenericPrompt(%q) = true, want false", specific)
			}
		})
	})
}

// Feature: mnemos-autopilot, Property related to TopicChanged: Verify Jaccard similarity properties.
// - Identical topics → TopicChanged returns false (similarity = 1.0 >= threshold)
// - Completely different topics (no shared words) → TopicChanged returns true (similarity = 0.0 < threshold)
// - Symmetric: TopicChanged(A, B, t) == TopicChanged(B, A, t)
func TestProp_TopicChanged_Jaccard(t *testing.T) {
	const threshold = 0.3

	// Generator for non-empty topic strings (space-separated words, no stop words)
	topicWords := []string{
		"auth", "middleware", "database", "pooling", "cache",
		"redis", "jwt", "token", "session", "handler",
		"router", "pipeline", "goroutine", "channel", "mutex",
		"interface", "struct", "pointer", "slice", "map",
	}

	topicGen := rapid.Custom(func(rt *rapid.T) string {
		n := rapid.IntRange(1, 4).Draw(rt, "word_count")
		words := make([]string, n)
		for i := range words {
			words[i] = rapid.SampledFrom(topicWords).Draw(rt, "word")
		}
		return strings.Join(words, " ")
	})

	t.Run("identical topics are not changed", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			topic := topicGen.Draw(rt, "topic")

			if hook.TopicChanged(topic, topic, threshold) {
				rt.Fatalf("TopicChanged(%q, %q, %.1f) = true, want false (identical topics should not be changed)",
					topic, topic, threshold)
			}
		})
	})

	t.Run("completely disjoint topics are changed", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			// Build two topics with guaranteed no shared words by using two disjoint word sets
			setA := []string{"auth", "jwt", "token", "session", "login"}
			setB := []string{"database", "pooling", "query", "schema", "migration"}

			nA := rapid.IntRange(1, 3).Draw(rt, "words_a")
			nB := rapid.IntRange(1, 3).Draw(rt, "words_b")

			topicA := strings.Join(setA[:nA], " ")
			topicB := strings.Join(setB[:nB], " ")

			if !hook.TopicChanged(topicA, topicB, threshold) {
				rt.Fatalf("TopicChanged(%q, %q, %.1f) = false, want true (disjoint topics should be changed)",
					topicA, topicB, threshold)
			}
		})
	})

	t.Run("symmetry: TopicChanged(A,B) == TopicChanged(B,A)", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			topicA := topicGen.Draw(rt, "topic_a")
			topicB := topicGen.Draw(rt, "topic_b")

			ab := hook.TopicChanged(topicA, topicB, threshold)
			ba := hook.TopicChanged(topicB, topicA, threshold)

			if ab != ba {
				rt.Fatalf("TopicChanged is not symmetric: TopicChanged(%q, %q) = %v but TopicChanged(%q, %q) = %v",
					topicA, topicB, ab, topicB, topicA, ba)
			}
		})
	})
}
