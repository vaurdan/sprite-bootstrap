# sprite-bootstrap

A cross-platform CLI utility that bootstraps Sprite environments for IDE remote development.

## Features

- **Cross-platform**: Works on Linux, macOS, and Windows
- **Multiple IDEs**: Supports Zed, VS Code, and easily extensible for more
- **Secure**: Generates per-sprite SSH keys with proper permissions
- **Simple**: Single command to set up remote development

## Installation

### From Source

```bash
go build -o sprite-bootstrap
```

### Cross-compile

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o dist/sprite-bootstrap-linux-amd64

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o dist/sprite-bootstrap-darwin-amd64

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o dist/sprite-bootstrap-darwin-arm64

# Windows
GOOS=windows GOARCH=amd64 go build -o dist/sprite-bootstrap-windows-amd64.exe
```

## Usage

### Bootstrap for Zed

```bash
sprite-bootstrap zed -s mysprite -o myorg
```

### Bootstrap for VS Code

```bash
sprite-bootstrap vscode -s mysprite -o myorg
```

### Check Status

```bash
sprite-bootstrap status -s mysprite
```

### Stop Proxy

```bash
sprite-bootstrap stop -s mysprite
```

## Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--sprite` | `-s` | Sprite name | (required) |
| `--org` | `-o` | Organization | (optional) |
| `--port` | `-p` | Local SSH port | 2222 |
| `--help` | `-h` | Show help | |

## How It Works

1. **SSH Key Generation**: Creates an ed25519 SSH key pair stored in `~/.sprite-bootstrap/keys/`
2. **Sprite Configuration**: Deploys the public key to the sprite's authorized_keys
3. **Bashrc Fix**: Adds an interactive check to prevent shell issues with IDEs
4. **Proxy Start**: Launches `sprite proxy` to tunnel SSH traffic
5. **Instructions**: Displays IDE-specific connection instructions

## Architecture

```
Local Machine                          Sprite VM
┌─────────────────────┐               ┌─────────────────────┐
│ sprite-bootstrap    │               │                     │
│                     │  sprite exec  │  - sshd (port 22)   │
│ ~/.sprite-bootstrap/│──────────────▶│  - authorized_keys  │
│   keys/sprite-key   │               │  - .bashrc fix      │
│                     │               │                     │
│ sprite proxy        │               │                     │
│ localhost:2222 ─────│──────────────▶│:22                  │
└─────────────────────┘               └─────────────────────┘
```

## Adding New IDE Support

To add a new IDE (e.g., Cursor), create `internal/tools/cursor.go`:

```go
package tools

import (
    "context"
    "fmt"
)

func init() {
    Register(&Cursor{})
}

type Cursor struct{}

func (c *Cursor) Name() string        { return "cursor" }
func (c *Cursor) Description() string { return "Bootstrap Cursor remote development" }
func (c *Cursor) Setup(ctx context.Context, opts SetupOptions) error { return nil }
func (c *Cursor) Validate(ctx context.Context) error { return nil }

func (c *Cursor) Instructions(opts SetupOptions) string {
    return fmt.Sprintf(`
Cursor Remote Development Ready!

Connect via: ssh -i %s -p %d sprite@localhost
`, opts.KeyPath, opts.LocalPort)
}
```

That's it - the command is automatically registered.

## Requirements

- Go 1.21+
- `sprite` CLI installed and in PATH
- SSH daemon available on the sprite

## License

MIT
