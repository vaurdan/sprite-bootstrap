//go:build !windows

package tools

import (
	"fmt"
	"os"
	"os/exec"
)

// findZedPlatformSpecific checks platform-specific locations for Zed.
// On Unix, we rely on PATH and shell aliases, so this returns empty.
func findZedPlatformSpecific() string {
	return ""
}

// findZedFallback tries shell alias resolution on Unix systems.
func findZedFallback() (string, bool) {
	for _, name := range zedBinaryNames {
		if shellHasCommand(name) {
			return name, true // true = use shell
		}
	}
	return "", false
}

// shellHasCommand checks if a command exists in the shell (including aliases)
func shellHasCommand(name string) bool {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Use 'command -v' which works for aliases, functions, and binaries
	cmd := exec.Command(shell, "-i", "-c", fmt.Sprintf("command -v %s", name))
	return cmd.Run() == nil
}

// buildZedCommand builds the exec.Cmd to launch Zed
func buildZedCommand(zedCmd string, useShell bool, url string) *exec.Cmd {
	if useShell {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		// Use interactive shell to load aliases
		return exec.Command(shell, "-i", "-c", fmt.Sprintf("%s %q", zedCmd, url))
	}
	return exec.Command(zedCmd, url)
}
