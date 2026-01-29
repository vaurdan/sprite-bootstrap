package cmd

import (
	"context"
	"fmt"

	"sprite-bootstrap/internal/sprite"
	"sprite-bootstrap/internal/tools"

	"github.com/spf13/cobra"
)

var (
	spriteName string
	orgName    string
	localPort  int
)

var rootCmd = &cobra.Command{
	Use:   "sprite-bootstrap",
	Short: "Bootstrap Sprite environments for IDE remote development",
	Long: `sprite-bootstrap is a cross-platform CLI utility that configures Sprite
environments for IDE remote development via SSH.

It handles:
  - SSH key generation and deployment
  - Sprite proxy management
  - IDE-specific configuration`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&spriteName, "sprite", "s", "", "Sprite name")
	rootCmd.PersistentFlags().StringVarP(&orgName, "org", "o", "", "Organization")
	rootCmd.PersistentFlags().IntVarP(&localPort, "port", "p", 2222, "Local SSH port")

	// Register commands for all tools
	for _, tool := range tools.All() {
		rootCmd.AddCommand(makeToolCommand(tool))
	}
}

func makeToolCommand(tool tools.Tool) *cobra.Command {
	return &cobra.Command{
		Use:   tool.Name(),
		Short: tool.Description(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if spriteName == "" {
				return fmt.Errorf("sprite name required (-s)")
			}

			if err := sprite.CheckSpriteInstalled(); err != nil {
				return err
			}

			ctx := context.Background()
			opts := tools.NewSetupOptions(spriteName, orgName, localPort)
			return tools.Bootstrap(ctx, tool, opts)
		},
	}
}

func Execute() error {
	return rootCmd.Execute()
}
