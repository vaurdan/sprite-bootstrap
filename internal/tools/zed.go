package tools

import (
	"context"
	"fmt"
	"os/exec"
)

func init() {
	Register(&Zed{})
}

// Zed implements the Tool interface for Zed IDE
type Zed struct{}

func (z *Zed) Name() string {
	return "zed"
}

func (z *Zed) Description() string {
	return "Bootstrap Zed remote development"
}

// zedBinaryNames are the possible names for the Zed binary
var zedBinaryNames = []string{"zed", "zeditor", "zedit", "zed-editor"}

// findZedBinary finds the Zed binary in PATH
func findZedBinary() string {
	for _, name := range zedBinaryNames {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func (z *Zed) Setup(ctx context.Context, opts SetupOptions) error {
	return nil
}

func (z *Zed) Instructions(opts SetupOptions) string {
	sshURL := fmt.Sprintf("ssh://%s@localhost:%d/home/sprite", opts.SpriteName, opts.LocalPort)

	// Try to launch Zed
	if zedBin := findZedBinary(); zedBin != "" {
		cmd := exec.Command(zedBin, sshURL)
		if err := cmd.Start(); err == nil {
			cmd.Process.Release()
			return fmt.Sprintf(`
Zed Remote Development Ready!

Opening Zed with: %s

If Zed doesn't open, connect manually:
  %s %s
`, sshURL, zedBin, sshURL)
		}
	}

	return fmt.Sprintf(`
Zed Remote Development Ready!

Open Zed and connect to:
  %s

Or run:
  zed %s
`, sshURL, sshURL)
}

func (z *Zed) Validate(ctx context.Context) error {
	return nil
}
