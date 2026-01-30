package config

import (
	"encoding/json"
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

// Preferences stores user preferences
type Preferences struct {
	NeverAskClaudeCodeExtension bool `json:"never_ask_claude_code_extension,omitempty"`
}

// prefsFile returns the path to the preferences file
func prefsFile() string {
	return filepath.Join(StateDir(), "preferences.json")
}

// LoadPreferences loads user preferences from disk
func LoadPreferences() (*Preferences, error) {
	prefs := &Preferences{}
	data, err := os.ReadFile(prefsFile())
	if err != nil {
		if os.IsNotExist(err) {
			return prefs, nil // Return empty prefs if file doesn't exist
		}
		return prefs, err
	}
	if err := json.Unmarshal(data, prefs); err != nil {
		return prefs, err
	}
	return prefs, nil
}

// SavePreferences saves user preferences to disk
func SavePreferences(prefs *Preferences) error {
	if err := EnsureStateDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(prefsFile(), data, 0600)
}
