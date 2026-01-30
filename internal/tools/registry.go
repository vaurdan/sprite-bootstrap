package tools

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"sprite-bootstrap/internal/config"
	"sprite-bootstrap/internal/sshserver"

	"github.com/superfly/sprites-go"
)

// ANSI color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorBold   = "\033[1m"
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
	// Validate prerequisites
	if err := tool.Validate(ctx); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Wake up the sprite first (it might be in warm/sleep state)
	fmt.Printf("%s⏳%s Waking sprite %s%s%s...\n", ColorYellow, ColorReset, ColorCyan, opts.SpriteName, ColorReset)
	if err := wakeSprite(ctx, opts); err != nil {
		return fmt.Errorf("failed to wake sprite: %w", err)
	}
	fmt.Printf("%s✓%s Sprite ready\n", ColorGreen, ColorReset)

	// Ensure serve is running
	if !IsServeRunning() {
		fmt.Printf("%s⏳%s Starting SSH server...\n", ColorYellow, ColorReset)
		if err := StartServe(opts.LocalPort); err != nil {
			return fmt.Errorf("failed to start SSH server: %w", err)
		}
		fmt.Printf("%s✓%s SSH server listening on port %d\n", ColorGreen, ColorReset, opts.LocalPort)
	} else {
		fmt.Printf("%s✓%s SSH server listening on port %d\n", ColorGreen, ColorReset, opts.LocalPort)
	}

	// Tool-specific setup
	if err := tool.Setup(ctx, opts); err != nil {
		return fmt.Errorf("failed tool setup: %w", err)
	}

	// Print instructions
	fmt.Println(tool.Instructions(opts))

	return nil
}

// wakeSprite sends a simple command to wake up a sprite from warm/sleep state
func wakeSprite(ctx context.Context, opts SetupOptions) error {
	// Resolve token from sprites config
	tokenOpts := &sshserver.TokenOptions{
		Organization: opts.OrgName,
	}
	if err := tokenOpts.Resolve(); err != nil {
		return fmt.Errorf("failed to resolve sprites credentials: %w\nRun 'sprite login' first", err)
	}

	// Create sprites client
	client := sprites.New(tokenOpts.AuthToken, sprites.WithBaseURL(tokenOpts.API))

	// Get the sprite
	sprite, err := client.GetSprite(ctx, opts.SpriteName)
	if err != nil {
		return fmt.Errorf("sprite not found: %s", opts.SpriteName)
	}

	// Run a simple command to wake it up
	// Using 'true' as it's the simplest command that does nothing but succeed
	wakeCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := sprite.CommandContext(wakeCtx, "true")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to wake sprite: %w", err)
	}

	return nil
}

// NewSetupOptions creates SetupOptions from common parameters
func NewSetupOptions(spriteName, orgName string, localPort int) SetupOptions {
	return SetupOptions{
		SpriteName: spriteName,
		OrgName:    orgName,
		LocalPort:  localPort,
	}
}

// ServePidFile returns the path to the serve PID file
func ServePidFile() string {
	return filepath.Join(config.StateDir(), "serve.pid")
}

// isPortAvailable checks if a port is available for binding
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// StartServe starts the serve command in the background
func StartServe(port int) error {
	// Check if port is available
	if !isPortAvailable(port) {
		return fmt.Errorf("port %d is already in use by another service\nTry a different port with -p flag, e.g.: sprite-bootstrap zed -s mysprite -p 2223", port)
	}

	if err := config.EnsureStateDir(); err != nil {
		return err
	}

	// Get the path to ourselves
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(executable, "serve", "-l", fmt.Sprintf(":%d", port))
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start serve: %w", err)
	}

	// Save PID
	pidFile := ServePidFile()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to save PID: %w", err)
	}

	cmd.Process.Release()

	// Wait for server to be ready (port to be bound)
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isPortAvailable(port) {
			return nil // Server is now listening
		}
	}

	return fmt.Errorf("server started but failed to bind to port %d", port)
}

// StopServe stops the running serve process
func StopServe() error {
	pidFile := ServePidFile()

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		os.Remove(pidFile)
		return fmt.Errorf("invalid PID: %w", err)
	}

	if err := signalTerminate(pid); err != nil {
		os.Remove(pidFile)
		return nil
	}

	os.Remove(pidFile)
	return nil
}

// IsServeRunning checks if serve is running
func IsServeRunning() bool {
	pidFile := ServePidFile()

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}

	return isProcessRunning(pid)
}

// GetServePid returns the PID of the running serve, or 0 if not running
func GetServePid() int {
	pidFile := ServePidFile()

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0
	}

	if !isProcessRunning(pid) {
		return 0
	}

	return pid
}
