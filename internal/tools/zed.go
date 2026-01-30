package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
// 2. Platform-specific locations (Windows registry, common paths)
// 3. Direct binary lookup in PATH
// 4. Shell alias resolution (Unix only)
func findZedBinary() (string, bool) {
	// Check environment variable first
	if zedPath := os.Getenv("ZED_PATH"); zedPath != "" {
		return zedPath, false // false = don't use shell
	}

	// Check platform-specific locations
	if path := findZedPlatformSpecific(); path != "" {
		return path, false
	}

	// Try direct binary lookup
	for _, name := range zedBinaryNames {
		if path, err := exec.LookPath(name); err == nil {
			return path, false
		}
	}

	// Try platform-specific fallback (shell aliases on Unix)
	if name, useShell := findZedFallback(); name != "" {
		return name, useShell
	}

	return "", false
}

// launchZed launches Zed with the given URL
func launchZed(zedCmd string, useShell bool, url string) error {
	cmd := buildZedCommand(zedCmd, useShell, url)
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
%s%s✓ Zed Remote Development Ready!%s

%sOpening:%s %s

If Zed doesn't open, connect manually:
  %szed %s%s
`, ColorBold, ColorGreen, ColorReset, ColorCyan, ColorReset, sshURL, ColorYellow, sshURL, ColorReset)
		}
	}

	return fmt.Sprintf(`
%s%s✓ Zed Remote Development Ready!%s

%sConnect to:%s %s

Or run:
  %szed %s%s
`, ColorBold, ColorGreen, ColorReset, ColorCyan, ColorReset, sshURL, ColorYellow, sshURL, ColorReset)
}

func (z *Zed) Validate(ctx context.Context) error {
	return nil
}
