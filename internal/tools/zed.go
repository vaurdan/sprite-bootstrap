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
  ssh://sprite@localhost:%d/home/sprite

Or connect via SSH with:
  ssh -i %s -p %d sprite@localhost

Recommended: Add to ~/.ssh/config:

  Host sprite-%s
      HostName localhost
      Port %d
      User sprite
      IdentityFile %s
      StrictHostKeyChecking no
      UserKnownHostsFile /dev/null

Then in Zed use: ssh://sprite-%s/home/sprite
`, opts.LocalPort, opts.KeyPath, opts.LocalPort, opts.SpriteName, opts.LocalPort, opts.KeyPath, opts.SpriteName)
}

func (z *Zed) Validate(ctx context.Context) error {
	return nil
}
