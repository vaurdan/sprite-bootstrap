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
%s%s✓ VS Code Remote Development Ready!%s

1. Install the "Remote - SSH" extension in VS Code

2. In VS Code:
   - Press Cmd+Shift+P (or Ctrl+Shift+P)
   - Type "Remote-SSH: Connect to Host"
   - Enter: %s%s@localhost -p %d%s
`, ColorBold, ColorGreen, ColorReset, ColorYellow, opts.SpriteName, opts.LocalPort, ColorReset)
}

func (v *VSCode) Validate(ctx context.Context) error {
	return nil
}
