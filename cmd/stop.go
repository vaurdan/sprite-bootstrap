package cmd

import (
	"context"
	"fmt"
	"time"

	"sprite-bootstrap/internal/tools"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the SSH server",
	Long: `Stop the running SSH server.

If a sprite is specified with -s, also cleans up any lingering
IDE processes on that sprite.`,
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	// Clean up sprite if specified
	if spriteName != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := tools.CleanupSprite(ctx, spriteName, orgName); err != nil {
			fmt.Printf("%s⚠%s Cleanup warning: %v\n",
				tools.ColorYellow, tools.ColorReset, err)
		}
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
