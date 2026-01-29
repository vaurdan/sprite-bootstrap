package cmd

import (
	"fmt"

	"sprite-bootstrap/internal/tools"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the SSH server",
	Long:  `Stop the running SSH server.`,
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	if !tools.IsServeRunning() {
		fmt.Println("SSH server is not running")
		return nil
	}

	pid := tools.GetServePid()
	fmt.Printf("Stopping SSH server (PID %d)...\n", pid)

	if err := tools.StopServe(); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	fmt.Println("Server stopped.")
	return nil
}
