package cmd

import (
	"context"
	"fmt"
	"path"
	"strings"

	"sprite-bootstrap/internal/tools"

	"github.com/spf13/cobra"
)

var (
	spriteName string
	orgName    string
	localPort  int
	remotePath string
	version    = "dev"
)

// SetVersion sets the version string for the CLI
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

var rootCmd = &cobra.Command{
	Use:   "sprite-bootstrap",
	Short: "Bootstrap Sprite environments for IDE remote development",
	Long: `sprite-bootstrap is a cross-platform CLI utility that configures Sprite
environments for IDE remote development via SSH.

It runs a local SSH server that proxies connections to sprites.
Connect using: ssh <sprite-name>@localhost -p <port>`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&spriteName, "sprite", "s", "", "Sprite name")
	rootCmd.PersistentFlags().StringVarP(&orgName, "org", "o", "", "Organization")
	rootCmd.PersistentFlags().IntVarP(&localPort, "port", "p", 2222, "Local SSH port")
	rootCmd.PersistentFlags().StringVar(&remotePath, "path", "", "Remote path (relative to /home/sprite or absolute)")

	// Register commands for all tools
	for _, tool := range tools.All() {
		rootCmd.AddCommand(makeToolCommand(tool))
	}
}

// resolveRemotePath resolves the remote path, handling relative and absolute paths
func resolveRemotePath(p string) string {
	if p == "" {
		return "/home/sprite"
	}
	if strings.HasPrefix(p, "/") {
		return path.Clean(p)
	}
	return path.Join("/home/sprite", p)
}

func makeToolCommand(tool tools.Tool) *cobra.Command {
	return &cobra.Command{
		Use:   tool.Name(),
		Short: tool.Description(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if spriteName == "" {
				return fmt.Errorf("sprite name required (-s)")
			}

			ctx := context.Background()
			opts := tools.NewSetupOptions(spriteName, orgName, localPort, resolveRemotePath(remotePath))
			return tools.Bootstrap(ctx, tool, opts)
		},
	}
}

func Execute() error {
	return rootCmd.Execute()
}
