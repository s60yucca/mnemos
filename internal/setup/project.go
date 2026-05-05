package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// DeriveProjectID derives a project identifier from git remote URL or directory name.
// If userProvided is non-empty, it is used directly.
// Otherwise, attempts to extract from git remote URL, falling back to directory basename.
// The result is sanitized to lowercase with special characters replaced by hyphens.
func DeriveProjectID(userProvided string) (string, error) {
	if userProvided != "" {
		return SanitizeProjectID(userProvided), nil
	}

	// Try git remote URL
	projectID, err := deriveFromGit()
	if err == nil && projectID != "" {
		return SanitizeProjectID(projectID), nil
	}

	// Fallback to directory basename
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return SanitizeProjectID(filepath.Base(cwd)), nil
}

// deriveFromGit extracts the repository name from git remote URL.
// Returns empty string if not a git repo or git command fails.
func deriveFromGit() (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	url := strings.TrimSpace(string(output))
	if url == "" {
		return "", nil
	}

	// Extract repo name from URL
	// Examples:
	//   https://github.com/user/repo.git -> repo
	//   git@github.com:user/repo.git -> repo
	//   /path/to/repo -> repo

	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Get last path component
	parts := strings.FieldsFunc(url, func(r rune) bool {
		return r == '/' || r == ':'
	})

	if len(parts) == 0 {
		return "", nil
	}

	return parts[len(parts)-1], nil
}

// SanitizeProjectID converts a project ID to lowercase and replaces special characters.
func SanitizeProjectID(id string) string {
	id = strings.ToLower(id)
	// Replace non-alphanumeric characters (except hyphens) with hyphens
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	id = re.ReplaceAllString(id, "-")
	// Trim leading/trailing hyphens
	id = strings.Trim(id, "-")
	return id
}
