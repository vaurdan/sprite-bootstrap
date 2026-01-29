package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// StateDir returns the platform-appropriate state directory for sprite-bootstrap
func StateDir() string {
	switch runtime.GOOS {
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "sprite-bootstrap")
		}
		return filepath.Join(os.Getenv("USERPROFILE"), ".sprite-bootstrap")
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), ".sprite-bootstrap")
	default: // linux
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			return filepath.Join(xdg, "sprite-bootstrap")
		}
		return filepath.Join(os.Getenv("HOME"), ".sprite-bootstrap")
	}
}

// EnsureStateDir creates the state directory if it doesn't exist
func EnsureStateDir() error {
	return os.MkdirAll(StateDir(), 0700)
}
