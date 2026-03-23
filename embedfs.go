// Package mnemos provides the embedded templates filesystem.
// This file must live at the repo root so that //go:embed can reference
// the templates/ directory at the same level.
package mnemos

import "embed"

// Templates holds all files under the templates/ directory.
// The "all:" prefix is required to include dotfiles like .cursorrules.
//
//go:embed all:templates
var Templates embed.FS
