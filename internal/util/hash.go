package util

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ContentHash returns a SHA-256 hex hash of the normalized content.
// projectID is included so the same content in different projects is not considered a duplicate.
func ContentHash(content, projectID string) string {
	normalized := NormalizeContent(content) + "\x00" + strings.ToLower(strings.TrimSpace(projectID))
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h)
}

// NormalizeContent trims and lowercases content for consistent hashing
func NormalizeContent(content string) string {
	return strings.TrimSpace(strings.ToLower(content))
}
