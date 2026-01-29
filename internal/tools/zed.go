package tools

import (
	"context"
	"fmt"
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

func (z *Zed) Setup(ctx context.Context, opts SetupOptions) error {
	return nil
}

func (z *Zed) Instructions(opts SetupOptions) string {
	return fmt.Sprintf(`
Zed Remote Development Ready!

In Zed, use "Open Remote Project" (Cmd+Shift+P -> "remote") with:
  ssh://%s@localhost:%d/home/sprite

Or connect via SSH:
  ssh %s@localhost -p %d
`, opts.SpriteName, opts.LocalPort, opts.SpriteName, opts.LocalPort)
}

func (z *Zed) Validate(ctx context.Context) error {
	return nil
}
