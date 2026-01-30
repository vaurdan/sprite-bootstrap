package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/superfly/sprites-go"
)

func init() {
	Register(&VSCode{})
}

// VSCode implements the Tool interface for Visual Studio Code
type VSCode struct{}

func (v *VSCode) Name() string {
	return "vscode"
}

func (v *VSCode) Description() string {
	return "Bootstrap VS Code remote development"
}

const remoteSSHExtensionID = "ms-vscode-remote.remote-ssh"
const claudeCodeExtensionID = "anthropic.claude-code"

// Markers for our managed SSH config entries
const (
	sshConfigStartMarker = "# >>> sprite-bootstrap %s >>>"
	sshConfigEndMarker   = "# <<< sprite-bootstrap %s <<<"
)

// findVSCodeBinary finds the VS Code binary
func findVSCodeBinary() string {
	if codePath := os.Getenv("VSCODE_PATH"); codePath != "" {
		return codePath
	}
	if path, err := exec.LookPath("code"); err == nil {
		return path
	}
	return ""
}

// hasExtension checks if VS Code has a specific extension installed
func hasExtension(binary, extensionID string) bool {
	cmd := exec.Command(binary, "--list-extensions")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		if strings.EqualFold(strings.TrimSpace(scanner.Text()), extensionID) {
			return true
		}
	}
	return false
}

// installExtension installs a VS Code extension locally
func installExtension(binary, extensionID string) error {
	cmd := exec.Command(binary, "--install-extension", extensionID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installRemoteExtension installs a VS Code extension on a remote host
func installRemoteExtension(binary, remoteHost, extensionID string) error {
	remoteArg := fmt.Sprintf("ssh-remote+%s", remoteHost)
	cmd := exec.Command(binary, "--remote", remoteArg, "--install-extension", extensionID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// sshConfigHostName returns the SSH config host name for a sprite
func sshConfigHostName(spriteName string) string {
	return fmt.Sprintf("sprite-%s", spriteName)
}

// sshConfigPath returns the path to the user's SSH config
func sshConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".ssh", "config"), nil
}

// addSSHConfigEntry adds a sprite SSH config entry if not already present
func addSSHConfigEntry(opts SetupOptions) error {
	configPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	// Ensure .ssh directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}

	hostName := sshConfigHostName(opts.SpriteName)
	startMarker := fmt.Sprintf(sshConfigStartMarker, opts.SpriteName)
	endMarker := fmt.Sprintf(sshConfigEndMarker, opts.SpriteName)

	// Read existing config
	existingConfig, _ := os.ReadFile(configPath)

	// Check if entry already exists - if so, remove it first
	configStr := string(existingConfig)
	if strings.Contains(configStr, startMarker) {
		configStr = removeSSHConfigEntryFromString(configStr, opts.SpriteName)
	}

	// Build new entry
	entry := fmt.Sprintf(`%s
Host %s
    HostName localhost
    Port %d
    User %s
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
%s
`, startMarker, hostName, opts.LocalPort, opts.SpriteName, endMarker)

	// Append to config
	if len(configStr) > 0 && !strings.HasSuffix(configStr, "\n") {
		configStr += "\n"
	}
	configStr += entry

	return os.WriteFile(configPath, []byte(configStr), 0600)
}

// removeSSHConfigEntryFromString removes a sprite entry from the config string
func removeSSHConfigEntryFromString(config, spriteName string) string {
	startMarker := fmt.Sprintf(sshConfigStartMarker, spriteName)
	endMarker := fmt.Sprintf(sshConfigEndMarker, spriteName)

	lines := strings.Split(config, "\n")
	var result []string
	inBlock := false

	for _, line := range lines {
		if strings.TrimSpace(line) == startMarker {
			inBlock = true
			continue
		}
		if strings.TrimSpace(line) == endMarker {
			inBlock = false
			continue
		}
		if !inBlock {
			result = append(result, line)
		}
	}

	// Clean up extra blank lines at the end
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	if len(result) > 0 {
		return strings.Join(result, "\n") + "\n"
	}
	return ""
}

// removeSSHConfigEntry removes a sprite SSH config entry
func removeSSHConfigEntry(spriteName string) error {
	configPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	existingConfig, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	newConfig := removeSSHConfigEntryFromString(string(existingConfig), spriteName)
	return os.WriteFile(configPath, []byte(newConfig), 0600)
}

// launchVSCode launches VS Code with SSH remote connection
func launchVSCode(binary string, opts SetupOptions) error {
	hostName := sshConfigHostName(opts.SpriteName)
	remoteArg := fmt.Sprintf("ssh-remote+%s", hostName)

	remotePath := opts.RemotePath
	if !strings.HasSuffix(remotePath, "/") {
		remotePath += "/"
	}

	cmd := exec.Command(binary, "--remote", remoteArg, remotePath)
	if err := cmd.Start(); err != nil {
		return err
	}
	cmd.Process.Release()
	return nil
}

func (v *VSCode) Setup(ctx context.Context, opts SetupOptions) error {
	binary := findVSCodeBinary()
	if binary == "" {
		return nil
	}

	// Install Remote-SSH extension if needed
	if !hasExtension(binary, remoteSSHExtensionID) {
		fmt.Printf("%s⏳%s Installing Remote-SSH extension...\n", ColorYellow, ColorReset)
		if err := installExtension(binary, remoteSSHExtensionID); err != nil {
			fmt.Printf("%s⚠%s Failed to install extension: %v\n", ColorYellow, ColorReset, err)
		} else {
			fmt.Printf("%s✓%s Remote-SSH extension installed\n", ColorGreen, ColorReset)
		}
	}

	// Add SSH config entry
	if err := addSSHConfigEntry(opts); err != nil {
		fmt.Printf("%s⚠%s Failed to add SSH config: %v\n", ColorYellow, ColorReset, err)
	}

	// Kill any existing VS Code server processes on the sprite to ensure clean connection
	if opts.Sprite != nil {
		fmt.Printf("%s⏳%s Cleaning up existing VS Code server...\n", ColorYellow, ColorReset)
		if err := cleanupVSCodeServer(ctx, opts.Sprite); err != nil {
			// Log for debugging but don't fail - cleanup is best effort
			fmt.Printf("%s✓%s VS Code server cleanup done (note: %v)\n", ColorGreen, ColorReset, err)
		} else {
			fmt.Printf("%s✓%s VS Code server cleanup done\n", ColorGreen, ColorReset)
		}
	}

	return nil
}

// cleanupVSCodeServer kills any existing VS Code server processes on the sprite
func cleanupVSCodeServer(ctx context.Context, sprite *sprites.Sprite) error {
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Kill VS Code server and related processes
	// Use pgrep to find PIDs first, then kill them - this avoids pkill killing itself
	// (pkill -f 'vscode-server' would match its own command line containing that string)
	cmd := sprite.CommandContext(cleanupCtx,
		"/bin/bash", "-c",
		"pids=$(pgrep -f '[v]scode-server' 2>/dev/null); [ -n \"$pids\" ] && kill $pids 2>/dev/null; true",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*sprites.ExitError); ok {
			fmt.Printf("    [debug] cleanup exit code: %d\n", exitErr.ExitCode())
		} else {
			fmt.Printf("    [debug] cleanup error: %v\n", err)
		}
	}
	return err
}

func (v *VSCode) Instructions(opts SetupOptions) string {
	hostName := sshConfigHostName(opts.SpriteName)

	binary := findVSCodeBinary()
	if binary != "" {
		if err := launchVSCode(binary, opts); err == nil {
			// Install Claude Code extension on the remote in the background
			// Wait a bit for the SSH connection to establish first
			go func() {
				time.Sleep(5 * time.Second)
				fmt.Printf("\n%s⏳%s Installing Claude Code extension on remote...\n", ColorYellow, ColorReset)
				if err := installRemoteExtension(binary, hostName, claudeCodeExtensionID); err != nil {
					fmt.Printf("%s⚠%s Failed to install Claude Code extension: %v\n", ColorYellow, ColorReset, err)
					fmt.Printf("   You can install it manually: %scode --remote ssh-remote+%s --install-extension %s%s\n",
						ColorYellow, hostName, claudeCodeExtensionID, ColorReset)
				} else {
					fmt.Printf("%s✓%s Claude Code extension installed on remote\n", ColorGreen, ColorReset)
				}
			}()

			return fmt.Sprintf(`
%s%s✓ VS Code Remote Development Ready!%s

%sOpening:%s %s:%s

If VS Code doesn't connect, try manually:
  %scode --remote ssh-remote+%s %s%s
`, ColorBold, ColorGreen, ColorReset,
				ColorCyan, ColorReset, hostName, opts.RemotePath,
				ColorYellow, hostName, opts.RemotePath, ColorReset)
		}
	}

	return fmt.Sprintf(`
%s%s✓ VS Code Remote Development Ready!%s

1. Install the "Remote - SSH" extension in VS Code
   %scode --install-extension %s%s

2. Connect via command line:
   %scode --remote ssh-remote+%s %s%s

   Or in VS Code:
   - Press Cmd+Shift+P (or Ctrl+Shift+P)
   - Type "Remote-SSH: Connect to Host"
   - Select: %s%s%s

3. Install Claude Code on the remote:
   %scode --remote ssh-remote+%s --install-extension %s%s
`, ColorBold, ColorGreen, ColorReset,
		ColorYellow, remoteSSHExtensionID, ColorReset,
		ColorYellow, hostName, opts.RemotePath, ColorReset,
		ColorYellow, hostName, ColorReset,
		ColorYellow, hostName, claudeCodeExtensionID, ColorReset)
}

func (v *VSCode) Validate(ctx context.Context) error {
	return nil
}

// Cleanup implements the Cleaner interface for VSCode
func (v *VSCode) Cleanup(ctx context.Context, sprite *sprites.Sprite) error {
	spriteName := sprite.Name()
	if err := removeSSHConfigEntry(spriteName); err != nil {
		return fmt.Errorf("failed to remove SSH config entry: %w", err)
	}
	return nil
}
