package tools

import (
	"context"
	"fmt"
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

func (v *VSCode) Setup(ctx context.Context, opts SetupOptions) error {
	return nil
}

func (v *VSCode) Instructions(opts SetupOptions) string {
	return fmt.Sprintf(`
VS Code Remote Development Ready!

1. Install the "Remote - SSH" extension in VS Code

2. In VS Code:
   - Press Cmd+Shift+P (or Ctrl+Shift+P)
   - Type "Remote-SSH: Connect to Host"
   - Enter: %s@localhost -p %d

Or connect via SSH:
  ssh %s@localhost -p %d
`, opts.SpriteName, opts.LocalPort, opts.SpriteName, opts.LocalPort)
}

func (v *VSCode) Validate(ctx context.Context) error {
	return nil
}
