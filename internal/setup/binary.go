package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ResolveBinaryPath returns the absolute path of the mnemos binary.
// It tries os.Executable() first (the running binary), then falls back
// to exec.LookPath("mnemos"). Calls filepath.EvalSymlinks on the result
// to resolve any symlinks (e.g. /proc/self/exe on Linux, Homebrew symlinks).
// Returns an error only if both fail.
func ResolveBinaryPath() (string, error) {
	// Strategy 1: use the running binary's path
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			return resolved, nil
		}
		// EvalSymlinks failed but we still have the raw path — use it
		return exe, nil
	}

	// Strategy 2: look up on PATH
	if path, err := exec.LookPath("mnemos"); err == nil {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return resolved, nil
		}
		return path, nil
	}

	return "", fmt.Errorf("mnemos binary not found: os.Executable() failed and 'mnemos' not on PATH")
}
