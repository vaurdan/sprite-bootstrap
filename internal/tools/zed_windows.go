//go:build windows

package tools

import (
	"os"
	"os/exec"
	"path/filepath"

	winreg "golang.org/x/sys/windows/registry"
)

// Common Windows install locations for Zed
var windowsZedPaths = []string{
	filepath.Join(os.Getenv("LOCALAPPDATA"), "Zed", "zed.exe"),
	filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Zed", "zed.exe"),
	filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Zed", "Zed.exe"),
	filepath.Join(os.Getenv("PROGRAMFILES"), "Zed", "zed.exe"),
	filepath.Join(os.Getenv("PROGRAMFILES"), "Zed", "Zed.exe"),
	filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Zed", "zed.exe"),
}

// Registry keys where Zed might register itself
var registryLocations = []struct {
	key    winreg.Key
	subkey string
	value  string
}{
	{winreg.CURRENT_USER, `Software\Zed Industries\Zed`, "InstallPath"},
	{winreg.CURRENT_USER, `Software\Zed`, "InstallPath"},
	{winreg.LOCAL_MACHINE, `Software\Zed Industries\Zed`, "InstallPath"},
	{winreg.LOCAL_MACHINE, `Software\Zed`, "InstallPath"},
	// App Paths registration
	{winreg.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\App Paths\zed.exe`, ""},
	{winreg.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\App Paths\zed.exe`, ""},
}

// findZedPlatformSpecific checks Windows-specific locations for Zed.
func findZedPlatformSpecific() string {
	// Check registry first
	if path := findZedInRegistry(); path != "" {
		return path
	}

	// Check common install paths
	for _, path := range windowsZedPaths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// findZedInRegistry searches the Windows registry for Zed's install location
func findZedInRegistry() string {
	for _, loc := range registryLocations {
		key, err := winreg.OpenKey(loc.key, loc.subkey, winreg.QUERY_VALUE)
		if err != nil {
			continue
		}
		defer key.Close()

		var path string
		if loc.value == "" {
			// Default value (for App Paths)
			path, _, err = key.GetStringValue("")
		} else {
			path, _, err = key.GetStringValue(loc.value)
		}
		if err != nil {
			continue
		}

		// If it's an InstallPath, append zed.exe
		if loc.value == "InstallPath" {
			path = filepath.Join(path, "zed.exe")
		}

		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// findZedFallback on Windows returns empty (no shell aliases)
func findZedFallback() (string, bool) {
	return "", false
}

// buildZedCommand builds the exec.Cmd to launch Zed on Windows
func buildZedCommand(zedCmd string, useShell bool, url string) *exec.Cmd {
	// On Windows, we don't use shell aliases
	return exec.Command(zedCmd, url)
}
