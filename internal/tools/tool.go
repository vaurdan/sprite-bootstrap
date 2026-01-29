package tools

import "context"

// Tool defines the interface each IDE bootstrap module must implement
type Tool interface {
	// Name returns the tool identifier (e.g., "zed", "vscode")
	Name() string

	// Description returns a short description for CLI help
	Description() string

	// Setup performs the IDE-specific setup on the sprite
	Setup(ctx context.Context, opts SetupOptions) error

	// Instructions returns connection instructions for the user
	Instructions(opts SetupOptions) string

	// Validate checks if prerequisites are met (e.g., local IDE installed)
	Validate(ctx context.Context) error
}

// SetupOptions contains configuration for setting up a tool
type SetupOptions struct {
	SpriteName string
	OrgName    string
	LocalPort  int
	KeyPath    string
}
