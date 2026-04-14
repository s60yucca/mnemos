package autopilot

import (
	"context"
	"strings"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
)

var (
	positiveSignals = []string{"works", "enabled", "use", "add", "fixed", "correct", "should"}
	negativeSignals = []string{"broken", "disabled", "don't use", "remove", "bug", "wrong", "avoid", "deprecated"}
)

// ContradictionDetector finds pairs of memories sharing 3+ entities with opposing sentiment signals.
type ContradictionDetector struct {
	mnemos *core.Mnemos
	cfg    config.AutopilotConfig
}

// NewContradictionDetector constructs a ContradictionDetector.
func NewContradictionDetector(mnemos *core.Mnemos, cfg config.AutopilotConfig) *ContradictionDetector {
	return &ContradictionDetector{mnemos: mnemos, cfg: cfg}
}

// Name returns the detector identifier.
func (d *ContradictionDetector) Name() string { return "contradiction" }

// containsSignalNearEntity checks if any signal word appears within 10 words of any entity mention.
// Multi-word signals (e.g. "don't use") are matched as a substring of the joined window.
func containsSignalNearEntity(content string, entities []string, signals []string) bool {
	words := strings.Fields(content)
	lower := strings.ToLower(content)

	for _, entity := range entities {
		entityLower := strings.ToLower(entity)
		// Find word positions where the entity appears as a substring
		for i, w := range words {
			if strings.Contains(strings.ToLower(w), entityLower) || strings.Contains(entityLower, strings.ToLower(w)) {
				// Check if the entity (possibly multi-word) appears near this position
				// Use a broader check: does the entity appear as substring in the content near this word?
				_ = lower // used below
				// Build the window of words around position i
				start := i - 10
				if start < 0 {
					start = 0
				}
				end := i + 11
				if end > len(words) {
					end = len(words)
				}
				window := strings.ToLower(strings.Join(words[start:end], " "))
				for _, sig := range signals {
					if strings.Contains(window, strings.ToLower(sig)) {
						return true
					}
				}
			}
		}
		// Also handle multi-word entities: find the entity as a substring in content
		// and check the surrounding word window
		if strings.Contains(lower, entityLower) {
			// Find the byte position and map to word index
			idx := strings.Index(lower, entityLower)
			// Count words before this position
			prefix := lower[:idx]
			wordsBefore := len(strings.Fields(prefix))
			start := wordsBefore - 10
			if start < 0 {
				start = 0
			}
			entityWordCount := len(strings.Fields(entity))
			end := wordsBefore + entityWordCount + 10
			if end > len(words) {
				end = len(words)
			}
			window := strings.ToLower(strings.Join(words[start:end], " "))
			for _, sig := range signals {
				if strings.Contains(window, strings.ToLower(sig)) {
					return true
				}
			}
		}
	}
	return false
}

// findOpposingSignals returns "positive_signal vs negative_signal" pairs where
// contentA has a positive signal near an entity and contentB has a negative signal (or vice versa).
func findOpposingSignals(contentA, contentB string, entities []string) []string {
	var pairs []string
	for _, entity := range entities {
		aHasPos := containsSignalNearEntity(contentA, []string{entity}, positiveSignals)
		aHasNeg := containsSignalNearEntity(contentA, []string{entity}, negativeSignals)
		bHasPos := containsSignalNearEntity(contentB, []string{entity}, positiveSignals)
		bHasNeg := containsSignalNearEntity(contentB, []string{entity}, negativeSignals)

		if aHasPos && bHasNeg {
			// Find the specific signals
			posFound := findFirstSignal(contentA, entity, positiveSignals)
			negFound := findFirstSignal(contentB, entity, negativeSignals)
			if posFound != "" && negFound != "" {
				pairs = append(pairs, posFound+" vs "+negFound)
			}
		} else if aHasNeg && bHasPos {
			negFound := findFirstSignal(contentA, entity, negativeSignals)
			posFound := findFirstSignal(contentB, entity, positiveSignals)
			if posFound != "" && negFound != "" {
				pairs = append(pairs, posFound+" vs "+negFound)
			}
		}
	}
	return pairs
}

// findFirstSignal returns the first signal from signals that appears near entity in content.
func findFirstSignal(content, entity string, signals []string) string {
	words := strings.Fields(content)
	lower := strings.ToLower(content)
	entityLower := strings.ToLower(entity)

	// Find entity position in words
	entityPos := -1
	for i, w := range words {
		if strings.Contains(strings.ToLower(w), entityLower) {
			entityPos = i
			break
		}
	}
	if entityPos == -1 {
		// Try substring match
		idx := strings.Index(lower, entityLower)
		if idx >= 0 {
			prefix := lower[:idx]
			entityPos = len(strings.Fields(prefix))
		}
	}
	if entityPos == -1 {
		return ""
	}

	start := entityPos - 10
	if start < 0 {
		start = 0
	}
	end := entityPos + 11
	if end > len(words) {
		end = len(words)
	}
	window := strings.ToLower(strings.Join(words[start:end], " "))

	for _, sig := range signals {
		if strings.Contains(window, strings.ToLower(sig)) {
			return sig
		}
	}
	return ""
}

// Detect finds memory pairs with opposing sentiment signals around shared entities.
func (d *ContradictionDetector) Detect(ctx context.Context, projectID string, maps EntityMaps) ([]Finding, error) {
	if !d.cfg.ContradictionEnabled {
		return []Finding{}, nil
	}

	// Invert entity map: entity → []memoryID
	entityIndex := make(map[string][]string)
	for memID, entities := range maps.Entities {
		for _, entity := range entities {
			entityIndex[entity] = append(entityIndex[entity], memID)
		}
	}

	// Gate 1: find pairs sharing 3+ entities
	// Count co-occurrences: pair → shared entity list
	type pairShared struct {
		a, b     string
		entities []string
	}
	pairEntities := make(map[pairKey][]string)

	for entity, memIDs := range entityIndex {
		if len(memIDs) < 2 {
			continue
		}
		for i := 0; i < len(memIDs); i++ {
			for j := i + 1; j < len(memIDs); j++ {
				a, b := normalizePair(memIDs[i], memIDs[j])
				key := pairKey{a, b}
				pairEntities[key] = append(pairEntities[key], entity)
			}
		}
	}

	// Collect qualifying pairs (3+ shared entities), capped at MaxContradictionPairs
	var qualifyingPairs []pairShared
	for key, entities := range pairEntities {
		if len(entities) < 3 {
			continue
		}
		qualifyingPairs = append(qualifyingPairs, pairShared{a: key.a, b: key.b, entities: entities})
		if len(qualifyingPairs) >= d.cfg.MaxContradictionPairs {
			break
		}
	}

	var findings []Finding

	for _, pair := range qualifyingPairs {
		sharedEntities := pair.entities

		// Gate 2: overlap ratio
		lenA := len(maps.Entities[pair.a])
		lenB := len(maps.Entities[pair.b])
		maxLen := lenA
		if lenB > maxLen {
			maxLen = lenB
		}
		if maxLen == 0 {
			continue
		}
		overlapScore := float64(len(sharedEntities)) / float64(maxLen)
		if overlapScore < d.cfg.ContradictionThreshold {
			continue
		}

		// Gate 3: opposing signals — fetch memories A and B
		mems, err := d.mnemos.GetByIDs(ctx, []string{pair.a, pair.b})
		if err != nil {
			continue
		}
		memA, okA := mems[pair.a]
		memB, okB := mems[pair.b]
		if !okA || !okB {
			continue
		}

		opposingSignals := findOpposingSignals(memA.Content, memB.Content, sharedEntities)
		if len(opposingSignals) == 0 {
			continue
		}

		findings = append(findings, Finding{
			Type: FindingPotentialContradiction,
			Metadata: map[string]any{
				"memory_a":         pair.a,
				"memory_b":         pair.b,
				"shared_entities":  sharedEntities,
				"overlap_score":    overlapScore,
				"opposing_signals": opposingSignals,
			},
		})
	}

	return findings, nil
}
