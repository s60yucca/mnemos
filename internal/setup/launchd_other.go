//go:build !darwin

package setup

// LaunchdPlistPath returns an empty string on non-darwin platforms.
func LaunchdPlistPath() (string, error) {
	return "", nil
}

// InstallLaunchdPlist is a no-op on non-darwin platforms.
func InstallLaunchdPlist(binPath string) error {
	return nil
}

// UninstallLaunchdPlist is a no-op on non-darwin platforms.
func UninstallLaunchdPlist() error {
	return nil
}
