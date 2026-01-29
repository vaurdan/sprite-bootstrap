package cmd

import (
	"fmt"

	"sprite-bootstrap/internal/sprite"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the proxy for a sprite",
	Long: `Stop the running sprite proxy.

This will terminate the SSH tunnel but does not remove SSH keys
or configuration from the sprite.`,
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	if spriteName == "" {
		return fmt.Errorf("sprite name required (-s)")
	}

	if !sprite.IsProxyRunning(spriteName) {
		fmt.Printf("No proxy running for sprite: %s\n", spriteName)
		return nil
	}

	pid := sprite.GetProxyPid(spriteName)
	fmt.Printf("Stopping proxy (PID %d) for sprite: %s...\n", pid, spriteName)

	if err := sprite.StopProxy(spriteName); err != nil {
		return fmt.Errorf("failed to stop proxy: %w", err)
	}

	fmt.Println("Proxy stopped.")
	return nil
}
