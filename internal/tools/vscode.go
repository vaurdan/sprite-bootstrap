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

	"sprite-bootstrap/internal/config"

	"github.com/charmbracelet/huh"
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


	return nil
}

// isClaudeCodeInstalledOnRemote checks if Claude Code extension is installed on the sprite
func isClaudeCodeInstalledOnRemote(ctx context.Context, sprite *sprites.Sprite) bool {
	if sprite == nil {
		return false
	}

	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if the extension directory exists in ~/.vscode-server/extensions/
	cmd := sprite.CommandContext(checkCtx,
		"/bin/bash", "-c",
		"ls -d ~/.vscode-server/extensions/anthropic.claude-code-* 2>/dev/null | head -1",
	)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

// installClaudeCodeOnRemote downloads and installs the Claude Code extension on the sprite
func installClaudeCodeOnRemote(ctx context.Context, sprite *sprites.Sprite) error {
	if sprite == nil {
		return fmt.Errorf("sprite is nil")
	}

	installCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// Download VSIX from VS Code marketplace and extract to extensions directory
	// The VSIX is a zip file that needs to be extracted to ~/.vscode-server/extensions/
	script := `
set -e
PUBLISHER="anthropic"
EXTENSION="claude-code"
EXT_DIR="$HOME/.vscode-server/extensions"

# Create extensions directory if needed
mkdir -p "$EXT_DIR"

# Get latest version from marketplace API
VERSION=$(curl -sf "https://marketplace.visualstudio.com/items?itemName=${PUBLISHER}.${EXTENSION}" | grep -oP '"version"\s*:\s*"\K[^"]+' | head -1)
if [ -z "$VERSION" ]; then
    # Fallback: try to get from Open VSX
    VERSION=$(curl -sf "https://open-vsx.org/api/${PUBLISHER}/${EXTENSION}" | grep -oP '"version"\s*:\s*"\K[^"]+' | head -1)
fi
if [ -z "$VERSION" ]; then
    echo "Could not determine extension version"
    exit 1
fi

echo "Installing ${PUBLISHER}.${EXTENSION} version ${VERSION}..."

# Check if already installed
if [ -d "$EXT_DIR/${PUBLISHER}.${EXTENSION}-${VERSION}" ]; then
    echo "Already installed"
    exit 0
fi

# Download VSIX from marketplace
VSIX_URL="https://${PUBLISHER}.gallery.vsassets.io/_apis/public/gallery/publisher/${PUBLISHER}/extension/${EXTENSION}/${VERSION}/assetbyname/Microsoft.VisualStudio.Services.VSIXPackage"
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

echo "Downloading from marketplace..."
if ! curl -sfL "$VSIX_URL" -o extension.vsix; then
    # Fallback to Open VSX
    echo "Trying Open VSX..."
    VSIX_URL="https://open-vsx.org/api/${PUBLISHER}/${EXTENSION}/${VERSION}/file/${PUBLISHER}.${EXTENSION}-${VERSION}.vsix"
    curl -sfL "$VSIX_URL" -o extension.vsix
fi

# Extract VSIX (it's a zip file)
unzip -q extension.vsix -d extracted

# Move extension to VS Code extensions directory
mv extracted/extension "$EXT_DIR/${PUBLISHER}.${EXTENSION}-${VERSION}"

# Cleanup
cd /
rm -rf "$TMP_DIR"

echo "Installed successfully"
`

	cmd := sprite.CommandContext(installCtx, "/bin/bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// promptInstallClaudeCode asks the user if they want to install Claude Code extension
// Returns: true = install, false = don't install
func promptInstallClaudeCode() bool {
	// Check preferences first
	prefs, _ := config.LoadPreferences()
	if prefs.NeverAskClaudeCodeExtension {
		return false
	}

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Install Claude Code extension on the remote?").
				Options(
					huh.NewOption("Yes, install it", "yes"),
					huh.NewOption("No, skip for now", "no"),
					huh.NewOption("Never ask again", "never"),
				).
				Value(&choice),
		),
	)

	err := form.Run()
	if err != nil {
		return false
	}

	switch choice {
	case "yes":
		return true
	case "never":
		prefs.NeverAskClaudeCodeExtension = true
		_ = config.SavePreferences(prefs)
		fmt.Printf("    %s(You can reset this in ~/.sprite-bootstrap/preferences.json)%s\n", ColorYellow, ColorReset)
		return false
	default:
		return false
	}
}

func (v *VSCode) Instructions(opts SetupOptions) string {
	hostName := sshConfigHostName(opts.SpriteName)
	ctx := context.Background()

	binary := findVSCodeBinary()
	if binary != "" {
		// Check if Claude Code extension is already installed on remote
		if opts.Sprite != nil && !isClaudeCodeInstalledOnRemote(ctx, opts.Sprite) {
			// Not installed - ask user if they want to install it
			if promptInstallClaudeCode() {
				fmt.Printf("%s⏳%s Installing Claude Code extension on remote...\n", ColorYellow, ColorReset)
				if err := installClaudeCodeOnRemote(ctx, opts.Sprite); err != nil {
					fmt.Printf("%s⚠%s Failed to install: %v\n", ColorYellow, ColorReset, err)
					fmt.Printf("   You can install it manually in VS Code Extensions\n")
				} else {
					fmt.Printf("%s✓%s Claude Code extension installed\n", ColorGreen, ColorReset)
				}
			}
		}

		// Launch VS Code
		if err := launchVSCode(binary, opts); err == nil {
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

3. Install Claude Code on the remote (optional):
   Search for "Claude Code" in VS Code Extensions
`, ColorBold, ColorGreen, ColorReset,
		ColorYellow, remoteSSHExtensionID, ColorReset,
		ColorYellow, hostName, opts.RemotePath, ColorReset,
		ColorYellow, hostName, ColorReset)
}

func (v *VSCode) Validate(ctx context.Context) error {
	return nil
}

// Cleanup implements the Cleaner interface for VSCode
func (v *VSCode) Cleanup(ctx context.Context, sprite *sprites.Sprite) error {
	spriteName := sprite.Name()

	// Remove SSH config entry
	if err := removeSSHConfigEntry(spriteName); err != nil {
		return fmt.Errorf("failed to remove SSH config entry: %w", err)
	}

	// Kill VS Code server processes on the sprite
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := sprite.CommandContext(cleanupCtx,
		"/bin/bash", "-c",
		"pids=$(pgrep -f '[v]scode-server' 2>/dev/null); [ -n \"$pids\" ] && kill $pids 2>/dev/null; true",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run() // Best effort - ignore errors

	return nil
}
