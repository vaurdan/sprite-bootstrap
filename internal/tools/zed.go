package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func init() {
	Register(&Zed{})
}

// Zed implements the Tool interface for Zed IDE
type Zed struct{}

func (z *Zed) Name() string {
	return "zed"
}

func (z *Zed) Description() string {
	return "Bootstrap Zed remote development"
}

// zedBinaryNames are the possible names for the Zed binary
var zedBinaryNames = []string{"zed", "zeditor", "zedit", "zed-editor"}

// findZedBinary finds the Zed binary, checking:
// 1. ZED_PATH environment variable
// 2. Direct binary lookup in PATH
// 3. Shell alias resolution
func findZedBinary() (string, bool) {
	// Check environment variable first
	if zedPath := os.Getenv("ZED_PATH"); zedPath != "" {
		return zedPath, false // false = don't use shell
	}

	// Try direct binary lookup
	for _, name := range zedBinaryNames {
		if path, err := exec.LookPath(name); err == nil {
			return path, false
		}
	}

	// Try to resolve via shell (for aliases)
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
		if runtime.GOOS == "windows" {
			return false
		}
		shell = "/bin/sh"
	}

	// Use 'command -v' which works for aliases, functions, and binaries
	cmd := exec.Command(shell, "-i", "-c", fmt.Sprintf("command -v %s", name))
	return cmd.Run() == nil
}

// launchZed launches Zed with the given URL
func launchZed(zedCmd string, useShell bool, url string) error {
	var cmd *exec.Cmd

	if useShell {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		// Use interactive shell to load aliases
		cmd = exec.Command(shell, "-i", "-c", fmt.Sprintf("%s %q", zedCmd, url))
	} else {
		cmd = exec.Command(zedCmd, url)
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	cmd.Process.Release()
	return nil
}

func (z *Zed) Setup(ctx context.Context, opts SetupOptions) error {
	return nil
}

func (z *Zed) Instructions(opts SetupOptions) string {
	sshURL := fmt.Sprintf("ssh://%s@localhost:%d/home/sprite", opts.SpriteName, opts.LocalPort)

	// Try to launch Zed
	if zedCmd, useShell := findZedBinary(); zedCmd != "" {
		if err := launchZed(zedCmd, useShell, sshURL); err == nil {
			return fmt.Sprintf(`
Zed Remote Development Ready!

Opening Zed with: %s

If Zed doesn't open, connect manually:
  zed %s

Tip: Set ZED_PATH environment variable if Zed isn't detected.
`, sshURL, sshURL)
		}
	}

	return fmt.Sprintf(`
Zed Remote Development Ready!

Open Zed and connect to:
  %s

Or run:
  zed %s

Tip: Set ZED_PATH=/path/to/zed if your Zed isn't detected.
`, sshURL, sshURL)
}

func (z *Zed) Validate(ctx context.Context) error {
	return nil
}
