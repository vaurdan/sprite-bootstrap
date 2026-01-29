package tools

import (
	"context"
	"fmt"

	"sprite-bootstrap/internal/config"
	"sprite-bootstrap/internal/sprite"
	"sprite-bootstrap/internal/ssh"
)

// registry holds all registered tools
var registry = make(map[string]Tool)

// Register adds a tool to the registry
func Register(tool Tool) {
	registry[tool.Name()] = tool
}

// Get returns a tool by name
func Get(name string) (Tool, bool) {
	tool, ok := registry[name]
	return tool, ok
}

// All returns all registered tools
func All() map[string]Tool {
	return registry
}

// Names returns all registered tool names
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Bootstrap performs the common bootstrap sequence for any tool
func Bootstrap(ctx context.Context, tool Tool, opts SetupOptions) error {
	client := sprite.NewClient(opts.SpriteName, opts.OrgName)

	// Validate prerequisites
	if err := tool.Validate(ctx); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	fmt.Printf("Setting up %s remote development...\n", tool.Name())

	// 1. Generate/ensure SSH key exists
	fmt.Printf("Ensuring SSH key at %s...\n", opts.KeyPath)
	publicKey, err := ssh.EnsureKey(opts.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to setup SSH key: %w", err)
	}

	// 2. Setup SSH on sprite (add key, fix permissions)
	fmt.Println("Configuring SSH on sprite...")
	if err := client.SetupSSH(ctx, publicKey); err != nil {
		return fmt.Errorf("failed to setup SSH on sprite: %w", err)
	}

	// 3. Fix .bashrc for non-interactive sessions
	fmt.Println("Fixing .bashrc for non-interactive sessions...")
	if err := client.FixBashrc(ctx); err != nil {
		return fmt.Errorf("failed to fix .bashrc: %w", err)
	}

	// 4. Ensure sshd is running
	fmt.Println("Ensuring SSH daemon is running...")
	if err := client.EnsureSSHD(ctx); err != nil {
		return fmt.Errorf("failed to ensure sshd: %w", err)
	}

	// 5. Tool-specific setup
	if err := tool.Setup(ctx, opts); err != nil {
		return fmt.Errorf("failed tool setup: %w", err)
	}

	// 6. Start proxy
	fmt.Printf("Starting proxy on localhost:%d...\n", opts.LocalPort)
	if err := sprite.StartProxy(opts.SpriteName, opts.OrgName, opts.LocalPort, 22); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	// 7. Print instructions
	fmt.Println(tool.Instructions(opts))

	return nil
}

// NewSetupOptions creates SetupOptions from common parameters
func NewSetupOptions(spriteName, orgName string, localPort int) SetupOptions {
	return SetupOptions{
		SpriteName: spriteName,
		OrgName:    orgName,
		LocalPort:  localPort,
		KeyPath:    config.KeyPath(spriteName),
	}
}
