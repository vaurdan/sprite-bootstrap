package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"sprite-bootstrap/internal/sshserver"

	"github.com/superfly/sprites-go"
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
	// Configure .zed/settings.json on the sprite for agent support
	return configureZedAgentSettings(ctx, opts)
}

// configureZedAgentSettings creates/updates .zed/settings.json on the sprite
// to enable Zed's built-in agent support (Claude, etc.)
func configureZedAgentSettings(ctx context.Context, opts SetupOptions) error {
	// Resolve token from sprites config
	tokenOpts := &sshserver.TokenOptions{
		Organization: opts.OrgName,
	}
	if err := tokenOpts.Resolve(); err != nil {
		// Non-fatal: agent config is optional
		return nil
	}

	// Create sprites client and get sprite
	client := sprites.New(tokenOpts.AuthToken, sprites.WithBaseURL(tokenOpts.API))
	sprite, err := client.GetSprite(ctx, opts.SpriteName)
	if err != nil {
		return nil // Non-fatal
	}

	setupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create .zed directory
	mkdirCmd := sprite.CommandContext(setupCtx, "mkdir", "-p", "/home/sprite/.zed")
	if err := mkdirCmd.Run(); err != nil {
		return nil // Non-fatal
	}

	// Read existing settings if present
	settingsPath := "/home/sprite/.zed/settings.json"
	var existingSettings map[string]any

	catCmd := sprite.CommandContext(setupCtx, "cat", settingsPath)
	var stdout bytes.Buffer
	catCmd.Stdout = &stdout
	if err := catCmd.Run(); err == nil && stdout.Len() > 0 {
		// Try to parse existing settings
		if err := json.Unmarshal(stdout.Bytes(), &existingSettings); err != nil {
			existingSettings = make(map[string]any)
		}
	} else {
		existingSettings = make(map[string]any)
	}

	// Check if agent settings already exist
	if _, hasAgent := existingSettings["agent"]; hasAgent {
		// Don't overwrite user's agent configuration
		return nil
	}

	// Add agent configuration for Zed's built-in Claude support
	// This enables the agent panel and uses Claude via Zed's integration
	existingSettings["agent"] = map[string]any{
		"default_profile": "claude",
		"profiles": map[string]any{
			"claude": map[string]any{},
		},
	}

	// Write updated settings
	settingsJSON, err := json.MarshalIndent(existingSettings, "", "  ")
	if err != nil {
		return nil // Non-fatal
	}

	teeCmd := sprite.CommandContext(setupCtx, "tee", settingsPath)
	teeCmd.Stdin = strings.NewReader(string(settingsJSON))
	var teeOut bytes.Buffer
	teeCmd.Stdout = &teeOut // Suppress tee output
	if err := teeCmd.Run(); err != nil {
		return nil // Non-fatal
	}

	fmt.Println("Configured Zed agent settings on sprite")
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

Agent: Zed's agent panel is pre-configured. Open it with cmd+shift+a (macOS) or ctrl+shift+a.

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

Agent: Zed's agent panel is pre-configured. Open it with cmd+shift+a (macOS) or ctrl+shift+a.

Tip: Set ZED_PATH=/path/to/zed if your Zed isn't detected.
`, sshURL, sshURL)
}

func (z *Zed) Validate(ctx context.Context) error {
	return nil
}
