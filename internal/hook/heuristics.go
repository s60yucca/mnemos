package hook

import (
	"strings"
	"unicode"
)

// genericSet contains prompts that are considered generic/non-specific.
var genericSet = map[string]bool{
	"continue":    true,
	"ok":          true,
	"okay":        true,
	"yes":         true,
	"no":          true,
	"sure":        true,
	"thanks":      true,
	"thank you":   true,
	"go ahead":    true,
	"proceed":     true,
	"next":        true,
	"keep going":  true,
	"looks good":  true,
	"lgtm":        true,
	"do it":       true,
	"go on":       true,
	"right":       true,
	"yep":         true,
	"yeah":        true,
	"nah":         true,
	"nope":        true,
	"fine":        true,
	"great":       true,
	"perfect":     true,
	"sounds good": true,
	"makes sense": true,
	"got it":      true,
	"understood":  true,
}

// stopWords contains words to remove during topic detection.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "have": true,
	"has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "could": true, "should": true, "can": true,
	"may": true, "might": true, "i": true, "you": true, "we": true,
	"they": true, "it": true, "this": true, "that": true, "these": true,
	"those": true, "to": true, "of": true, "in": true, "for": true,
	"on": true, "with": true, "at": true, "by": true, "from": true,
	"and": true, "or": true, "not": true, "but": true, "if": true,
	"then": true, "please": true, "let": true, "me": true, "us": true,
	"my": true, "our": true, "about": true, "just": true, "also": true,
	"very": true, "really": true, "actually": true, "basically": true,
	"probably": true, "want": true, "need": true, "think": true,
	"know": true, "look": true, "see": true, "try": true, "make": true,
	"work": true, "help": true, "using": true, "like": true,
}

// normalizePrompt lowercases, trims whitespace, and strips trailing punctuation.
func normalizePrompt(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	// Strip trailing punctuation
	s = strings.TrimRightFunc(s, func(r rune) bool {
		return unicode.IsPunct(r)
	})
	return strings.TrimSpace(s)
}

// IsGenericPrompt returns true if the prompt is a generic/non-specific response.
// Exact match only — no prefix matching.
func IsGenericPrompt(promptText string) bool {
	normalized := normalizePrompt(promptText)
	if normalized == "" {
		return false
	}

	// Check exact match against generic set
	if genericSet[normalized] {
		return true
	}

	// Check if text is <= 3 words AND all words are in generic set
	words := strings.Fields(normalized)
	if len(words) <= 3 {
		allGeneric := true
		for _, w := range words {
			if !genericSet[w] {
				allGeneric = false
				break
			}
		}
		if allGeneric {
			return true
		}
	}

	return false
}

// DetectTopic extracts the core topic from a prompt by removing stop words.
func DetectTopic(promptText string) string {
	// Lowercase
	s := strings.ToLower(promptText)

	// Remove punctuation except hyphens
	var sb strings.Builder
	for _, r := range s {
		if r == '-' || !unicode.IsPunct(r) {
			sb.WriteRune(r)
		}
	}
	s = sb.String()

	// Split into words
	words := strings.Fields(s)

	// Remove stop words and collect meaningful words
	meaningful := make([]string, 0, len(words))
	for _, w := range words {
		if !stopWords[w] && w != "" {
			meaningful = append(meaningful, w)
		}
	}

	// Take first 5 meaningful words
	if len(meaningful) > 5 {
		meaningful = meaningful[:5]
	}

	return strings.Join(meaningful, " ")
}

// toWordSet converts a topic string into a set of words.
func toWordSet(topic string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range strings.Fields(topic) {
		if w != "" {
			set[w] = true
		}
	}
	return set
}

// TopicChanged returns true if newTopic and activeTopic are sufficiently different,
// using Jaccard similarity on word sets.
func TopicChanged(newTopic, activeTopic string, threshold float64) bool {
	setA := toWordSet(newTopic)
	setB := toWordSet(activeTopic)

	// Count intersection
	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	// Count union
	union := len(setA)
	for w := range setB {
		if !setA[w] {
			union++
		}
	}

	// Both empty — treat as changed
	if union == 0 {
		return true
	}

	similarity := float64(intersection) / float64(union)
	return similarity < threshold
}
