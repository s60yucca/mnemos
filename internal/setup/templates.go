package setup

import (
	"fmt"
	"io/fs"

	mnemos "github.com/mnemos-dev/mnemos"
)

// GetTemplate returns the content of a template file by its path relative to
// the repo root (e.g. "templates/claude/CLAUDE.md").
// Returns an error if the file does not exist or cannot be read.
func GetTemplate(path string) (string, error) {
	data, err := fs.ReadFile(mnemos.Templates, path)
	if err != nil {
		return "", fmt.Errorf("template %q not found: %w", path, err)
	}
	return string(data), nil
}
