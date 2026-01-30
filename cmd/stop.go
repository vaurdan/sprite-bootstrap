package cmd

import (
	"context"
	"fmt"
	"time"

	"sprite-bootstrap/internal/sshserver"
	"sprite-bootstrap/internal/tools"

	"github.com/spf13/cobra"
	"github.com/superfly/sprites-go"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the SSH server",
	Long: `Stop the running SSH server.

If a sprite is specified with -s, also cleans up any lingering Zed
remote server processes on that sprite.`,
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	// Clean up sprite if specified
	if spriteName != "" {
		cleanupSprite(spriteName)
	}

	if !tools.IsServeRunning() {
		fmt.Println("SSH server is not running")
		return nil
	}

	pid := tools.GetServePid()
	fmt.Printf("%s⏳%s Stopping SSH server (PID %d)...\n", tools.ColorYellow, tools.ColorReset, pid)

	if err := tools.StopServe(); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	fmt.Printf("%s✓%s Server stopped\n", tools.ColorGreen, tools.ColorReset)
	return nil
}

// cleanupSprite removes lingering Zed processes from a sprite
func cleanupSprite(spriteName string) {
	fmt.Printf("%s⏳%s Cleaning up Zed processes on %s%s%s...\n",
		tools.ColorYellow, tools.ColorReset, tools.ColorCyan, spriteName, tools.ColorReset)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Resolve credentials
	tokenOpts := &sshserver.TokenOptions{
		Organization: orgName,
	}
	if err := tokenOpts.Resolve(); err != nil {
		fmt.Printf("%s⚠%s Could not connect to sprite for cleanup: %v\n",
			tools.ColorYellow, tools.ColorReset, err)
		return
	}

	// Connect to sprite
	client := sprites.New(tokenOpts.AuthToken, sprites.WithBaseURL(tokenOpts.API))
	sprite, err := client.GetSprite(ctx, spriteName)
	if err != nil {
		fmt.Printf("%s⚠%s Sprite not found: %s\n",
			tools.ColorYellow, tools.ColorReset, spriteName)
		return
	}

	// Kill Zed remote server processes
	// Using pkill with -f to match the process name pattern
	cleanupCmd := sprite.CommandContext(ctx, "pkill", "-f", "zed-remote-server")
	_ = cleanupCmd.Run() // Ignore errors - might not find any processes

	// Also clean up any orphaned proxy shells
	cleanupCmd = sprite.CommandContext(ctx, "pkill", "-f", "proxy.*--identifier")
	_ = cleanupCmd.Run()

	// Clean up Zed server state directory
	cleanupCmd = sprite.CommandContext(ctx, "rm", "-rf", "/home/sprite/.local/share/zed/server_state")
	_ = cleanupCmd.Run()

	fmt.Printf("%s✓%s Cleaned up Zed processes on %s\n",
		tools.ColorGreen, tools.ColorReset, spriteName)
}
