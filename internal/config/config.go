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

// KeyPath returns the path to the SSH private key for a sprite
func KeyPath(spriteName string) string {
	return filepath.Join(StateDir(), "keys", spriteName+"-key")
}

// PubKeyPath returns the path to the SSH public key for a sprite
func PubKeyPath(spriteName string) string {
	return KeyPath(spriteName) + ".pub"
}

// PidFile returns the path to the proxy PID file for a sprite
func PidFile(spriteName string) string {
	return filepath.Join(StateDir(), "pids", spriteName+"-proxy.pid")
}

// EnsureStateDir creates the state directory if it doesn't exist
func EnsureStateDir() error {
	return os.MkdirAll(StateDir(), 0700)
}

// EnsureKeysDir creates the keys directory if it doesn't exist
func EnsureKeysDir() error {
	return os.MkdirAll(filepath.Join(StateDir(), "keys"), 0700)
}

// EnsurePidsDir creates the pids directory if it doesn't exist
func EnsurePidsDir() error {
	return os.MkdirAll(filepath.Join(StateDir(), "pids"), 0700)
}
