//go:build darwin

package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const plistLabel = "com.mnemos.autopilot"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>             <string>{{.Label}}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{.BinPath}}</string>
    <string>autopilot</string>
    <string>run</string>
  </array>
  <key>StartInterval</key>     <integer>300</integer>
  <!-- TODO: read from ~/.mnemos/config.yaml -->
  <key>ThrottleInterval</key>  <integer>300</integer>
  <key>StandardOutPath</key>   <string>{{.LogDir}}/autopilot.out.log</string>
  <key>StandardErrorPath</key> <string>{{.LogDir}}/autopilot.err.log</string>
</dict>
</plist>
`

// LaunchdPlistPath returns the path to the mnemos autopilot launchd plist.
func LaunchdPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist"), nil
}

// InstallLaunchdPlist writes the autopilot launchd plist and bootstraps the service.
// Idempotent: runs bootout (ignoring error) then bootstrap.
func InstallLaunchdPlist(binPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}

	logDir := filepath.Join(home, ".mnemos", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	launchAgentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	plistPath := filepath.Join(launchAgentsDir, plistLabel+".plist")

	// Render plist template
	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return fmt.Errorf("parse plist template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		Label   string
		BinPath string
		LogDir  string
	}{
		Label:   plistLabel,
		BinPath: binPath,
		LogDir:  logDir,
	}); err != nil {
		return fmt.Errorf("render plist template: %w", err)
	}

	// Write plist file
	if err := os.WriteFile(plistPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)

	// Bootout first (ignore error — service may not be loaded)
	exec.Command("launchctl", "bootout", domain, plistPath).CombinedOutput() //nolint:errcheck

	// Bootstrap
	cmd := exec.Command("launchctl", "bootstrap", domain, plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap failed: %w\n%s", err, out)
	}

	return nil
}

// UninstallLaunchdPlist bootouts and deletes the plist.
// No-op if plist does not exist.
func UninstallLaunchdPlist() error {
	plistPath, err := LaunchdPlistPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return nil // no-op
	}

	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)

	// Bootout (ignore error — service may not be loaded)
	exec.Command("launchctl", "bootout", domain, plistPath).CombinedOutput() //nolint:errcheck

	// Delete plist
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}

	return nil
}
