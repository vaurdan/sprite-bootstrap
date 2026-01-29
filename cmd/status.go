package cmd

import (
	"fmt"

	"sprite-bootstrap/internal/tools"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SSH server status",
	Long:  `Display the current status of the SSH server.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("SSH Server Status")
	fmt.Println("─────────────────────────────────────")

	if tools.IsServeRunning() {
		pid := tools.GetServePid()
		fmt.Printf("Server:      ✓ running (PID %d) on port %d\n", pid, localPort)
		fmt.Println()
		fmt.Println("Connect with:")
		fmt.Printf("  ssh <sprite-name>@localhost -p %d\n", localPort)
	} else {
		fmt.Println("Server:      ✗ not running")
		fmt.Println()
		fmt.Println("Start with:")
		fmt.Println("  sprite-bootstrap serve")
		fmt.Println("Or:")
		fmt.Println("  sprite-bootstrap zed -s <sprite-name>")
	}

	return nil
}
