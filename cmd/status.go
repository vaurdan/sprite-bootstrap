package cmd

import (
	"fmt"

	"sprite-bootstrap/internal/config"
	"sprite-bootstrap/internal/sprite"
	"sprite-bootstrap/internal/ssh"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show bootstrap status for a sprite",
	Long: `Display the current bootstrap status for a sprite including:
  - SSH key status
  - Proxy status and PID
  - Connection information`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	if spriteName == "" {
		return fmt.Errorf("sprite name required (-s)")
	}

	keyPath := config.KeyPath(spriteName)
	keyExists := ssh.KeyExists(keyPath)
	proxyRunning := sprite.IsProxyRunning(spriteName)
	proxyPid := sprite.GetProxyPid(spriteName)

	fmt.Printf("Status for sprite: %s\n", spriteName)
	fmt.Println("─────────────────────────────────────")

	// SSH Key status
	fmt.Print("SSH Key:     ")
	if keyExists {
		fmt.Printf("✓ exists at %s\n", keyPath)
	} else {
		fmt.Println("✗ not found")
	}

	// Proxy status
	fmt.Print("Proxy:       ")
	if proxyRunning {
		fmt.Printf("✓ running (PID %d) on port %d\n", proxyPid, localPort)
	} else {
		fmt.Println("✗ not running")
	}

	// Connection info
	if keyExists && proxyRunning {
		fmt.Println()
		fmt.Println("Connection:")
		fmt.Printf("  ssh -i %s -p %d sprite@localhost\n", keyPath, localPort)
	}

	return nil
}
