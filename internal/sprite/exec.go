package sprite

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Client wraps interactions with the sprite CLI
type Client struct {
	SpriteName string
	OrgName    string
}

// NewClient creates a new sprite client
func NewClient(spriteName, orgName string) *Client {
	return &Client{
		SpriteName: spriteName,
		OrgName:    orgName,
	}
}

// Exec runs a command on the sprite via sprite exec
func (c *Client) Exec(ctx context.Context, command string) (string, error) {
	args := []string{"exec"}
	if c.OrgName != "" {
		args = append(args, "-o", c.OrgName)
	}
	if c.SpriteName != "" {
		args = append(args, "-s", c.SpriteName)
	}
	args = append(args, "--", "bash", "-c", command)

	cmd := exec.CommandContext(ctx, findSpriteBinary(), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("sprite exec failed: %w: %s", err, string(output))
	}
	return string(output), nil
}

// ExecQuiet runs a command and suppresses output unless there's an error
func (c *Client) ExecQuiet(ctx context.Context, command string) error {
	_, err := c.Exec(ctx, command)
	return err
}

// SetupSSH ensures SSH is properly configured on the sprite
func (c *Client) SetupSSH(ctx context.Context, publicKey string) error {
	// Ensure .ssh directory exists with correct permissions
	if _, err := c.Exec(ctx, "mkdir -p ~/.ssh && chmod 700 ~/.ssh"); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Add public key to authorized_keys if not already present
	escapedKey := strings.ReplaceAll(publicKey, "'", "'\\''")
	cmd := fmt.Sprintf(`
		key='%s'
		if ! grep -qF "$key" ~/.ssh/authorized_keys 2>/dev/null; then
			echo "$key" >> ~/.ssh/authorized_keys
		fi
		chmod 600 ~/.ssh/authorized_keys
	`, strings.TrimSpace(escapedKey))

	if _, err := c.Exec(ctx, cmd); err != nil {
		return fmt.Errorf("failed to add public key: %w", err)
	}

	return nil
}

// FixBashrc adds the interactive check to .bashrc to prevent issues with
// non-interactive sessions (required for Zed and other IDEs)
func (c *Client) FixBashrc(ctx context.Context) error {
	cmd := `
		if ! grep -q "sprite-bootstrap: interactive" ~/.bashrc 2>/dev/null; then
			temp=$(mktemp)
			echo '# sprite-bootstrap: interactive check
case $- in
    *i*) ;;
      *) return;;
esac
' | cat - ~/.bashrc > "$temp" 2>/dev/null || echo '# sprite-bootstrap: interactive check
case $- in
    *i*) ;;
      *) return;;
esac
' > "$temp"
			mv "$temp" ~/.bashrc
			chmod 644 ~/.bashrc
		fi
	`
	if _, err := c.Exec(ctx, cmd); err != nil {
		return fmt.Errorf("failed to fix .bashrc: %w", err)
	}
	return nil
}

// EnsureSSHD ensures the SSH daemon is running on the sprite
func (c *Client) EnsureSSHD(ctx context.Context) error {
	// Check if sshd is installed, install if not, then start
	cmd := `
		if ! which sshd > /dev/null 2>&1; then
			echo "Installing openssh-server..."
			sudo apt-get update -qq && sudo apt-get install -y -qq openssh-server
		fi
		if ! pgrep -x sshd > /dev/null 2>&1; then
			sudo /usr/sbin/sshd 2>/dev/null || sudo service ssh start 2>/dev/null || true
		fi
		pgrep -x sshd > /dev/null 2>&1
	`
	if _, err := c.Exec(ctx, cmd); err != nil {
		return fmt.Errorf("failed to ensure sshd is running: %w", err)
	}
	return nil
}

// findSpriteBinary returns the sprite binary name for the current platform
func findSpriteBinary() string {
	if runtime.GOOS == "windows" {
		return "sprite.exe"
	}
	return "sprite"
}

// CheckSpriteInstalled verifies the sprite CLI is available
func CheckSpriteInstalled() error {
	_, err := exec.LookPath(findSpriteBinary())
	if err != nil {
		return fmt.Errorf("sprite CLI not found in PATH. Please install sprite first")
	}
	return nil
}
