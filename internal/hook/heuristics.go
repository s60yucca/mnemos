package hook

import (
	"strings"
	"unicode"
)

// genericSet contains single-word prompts that are considered generic/non-specific.
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

// genericPrefixes are multi-word phrase starts that mark a prompt as generic.
// Prefix matching catches "can you help me fix X" → still generic opener even with content after.
var genericPrefixes = []string{
	"can you help",
	"help me",
	"please help",
	"what do you think",
	"let's start",
	"let's begin",
	"what next",
	"now what",
	"i see",
	"ok do it",
	"yes please",
	"no problem",
	"no worries",
	"never mind",
	"forget it",
	"ignore that",
	"that's fine",
	"that's great",
	"that's perfect",
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

// techKeywords is a curated set of common technical terms that get priority in topic detection.
var techKeywords = map[string]bool{
	"api": true, "jwt": true, "auth": true, "sdk": true, "config": true,
	"deploy": true, "pipeline": true, "database": true, "schema": true,
	"endpoint": true, "docker": true, "kubernetes": true, "gradle": true,
	"maven": true, "npm": true, "yarn": true, "webpack": true,
	"sql": true, "nosql": true, "redis": true, "postgres": true, "mysql": true,
	"sqlite": true, "mongo": true, "git": true, "ci": true, "cd": true,
	"ssl": true, "tls": true, "http": true, "https": true, "grpc": true,
	"rest": true, "graphql": true, "websocket": true, "socket": true,
	"token": true, "oauth": true, "session": true, "cookie": true,
	"middleware": true, "handler": true, "router": true, "controller": true,
	"service": true, "repository": true, "model": true, "migration": true,
	"test": true, "mock": true, "fixture": true, "lint": true, "build": true,
	"release": true, "tag": true, "branch": true, "merge": true, "rebase": true,
	"crash": true, "panic": true, "leak": true, "race": true, "deadlock": true,
	"goroutine": true, "thread": true, "async": true, "await": true, "callback": true,
	"hook": true, "event": true, "listener": true, "subscriber": true,
	"android": true, "ios": true, "kotlin": true, "swift": true, "flutter": true,
	"react": true, "vue": true, "angular": true, "node": true, "golang": true,
	"python": true, "java": true, "typescript": true, "javascript": true,
	// Common technical-adjacent terms
	"authentication": true, "authorization": true, "encryption": true,
	"refactor": true, "implement": true, "optimize": true, "debug": true,
	"performance": true, "latency": true, "throughput": true, "memory": true,
	"query": true, "index": true, "cache": true, "caching": true,
	"error": true, "exception": true, "timeout": true, "retry": true,
	"logging": true, "metrics": true, "tracing": true, "monitoring": true,
	"payload": true, "request": true, "response": true, "header": true,
	"function": true, "method": true, "struct": true, "interface": true,
	"channel": true, "mutex": true, "concurrent": true,
}

// normalizePrompt lowercases, trims whitespace, and strips trailing punctuation.
func normalizePrompt(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	s = strings.TrimRightFunc(s, func(r rune) bool {
		return unicode.IsPunct(r)
	})
	return strings.TrimSpace(s)
}

// isTechnicalTerm returns true when a word looks like a technical identifier:
// contains code-oriented characters (dots, underscores, slashes), is camelCase/PascalCase,
// or is in the curated techKeywords list.
func isTechnicalTerm(word string) bool {
	if word == "" {
		return false
	}
	// Known technical keywords (lowercase comparison)
	if techKeywords[strings.ToLower(word)] {
		return true
	}
	// Contains characters typical of identifiers and paths
	if strings.ContainsAny(word, "._/\\:@#$") {
		return true
	}
	// camelCase or PascalCase: has both upper and lower letters
	hasUpper := false
	hasLower := false
	for _, r := range word {
		if unicode.IsUpper(r) {
			hasUpper = true
		} else if unicode.IsLower(r) {
			hasLower = true
		}
		if hasUpper && hasLower {
			return true
		}
	}
	return false
}

// extractTechnicalTerms returns only the technical words from a slice.
func extractTechnicalTerms(words []string) []string {
	tech := make([]string, 0, len(words))
	for _, w := range words {
		if isTechnicalTerm(w) {
			tech = append(tech, w)
		}
	}
	return tech
}

// IsGenericPrompt returns true if the prompt carries no task-specific information.
// Checks: empty/single-word, exact-match generic set, multi-word generic prefix,
// and short all-generic-word prompts.
func IsGenericPrompt(promptText string) bool {
	normalized := normalizePrompt(promptText)
	if normalized == "" {
		return true
	}

	// Single word — always generic (even "fix" alone is not actionable)
	words := strings.Fields(normalized)
	if len(words) == 1 {
		return true
	}

	// Exact match against generic set
	if genericSet[normalized] {
		return true
	}

	// Short (≤3 word) prompts where all words are in the generic set
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

	// Multi-word generic prefix match (catches "can you help me fix X" patterns)
	for _, prefix := range genericPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}

	return false
}

// DetectTopic extracts the core topic from a prompt.
// Strategy: if there are ≥2 technical terms, prefer those (they carry more
// signal for memory search). Otherwise, fall back to first 4 non-stopword words.
// Returns empty string when the result would be too vague to search.
func DetectTopic(promptText string) string {
	// Lowercase, preserve hyphens (e.g. "build-time" → meaningful), strip other punct
	s := strings.ToLower(promptText)
	var sb strings.Builder
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' || !unicode.IsPunct(r) {
			sb.WriteRune(r)
		}
	}
	s = sb.String()

	words := strings.Fields(s)

	// Remove stop words, collect meaningful tokens
	meaningful := make([]string, 0, len(words))
	for _, w := range words {
		if !stopWords[w] && w != "" {
			meaningful = append(meaningful, w)
		}
	}

	if len(meaningful) < 2 {
		return "" // too little signal — caller will skip searching
	}

	// Prefer technical terms — they make better search queries
	tech := extractTechnicalTerms(meaningful)
	if len(tech) >= 2 {
		if len(tech) > 4 {
			tech = tech[:4]
		}
		return strings.Join(tech, " ")
	}

	// Fall back: first 4 meaningful words
	if len(meaningful) > 4 {
		meaningful = meaningful[:4]
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

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

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
