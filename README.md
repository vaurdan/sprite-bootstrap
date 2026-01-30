# sprite-bootstrap

A cross-platform CLI utility that bootstraps Sprite environments for IDE remote development.

## Features

- **Cross-platform**: Works on Linux, macOS, and Windows
- **No sshd required**: Uses the sprites API directly, no need to install SSH server on sprites
- **Multiple IDEs**: Supports Zed, VS Code, and easily extensible for more
- **Simple**: Single server handles all sprites

## Installation

### Quick Install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/vaurdan/sprite-bootstrap/main/install.sh | sh
```

The script automatically installs to:
- **macOS**: `/usr/local/bin` (may prompt for sudo)
- **Linux**: `~/.local/bin`

Override with `INSTALL_DIR`:

```bash
curl -fsSL https://raw.githubusercontent.com/vaurdan/sprite-bootstrap/main/install.sh | INSTALL_DIR=~/.bin sh
```

### Manual Download

Download the latest binary from [GitHub Releases](https://github.com/vaurdan/sprite-bootstrap/releases):

| Platform | Binary |
|----------|--------|
| Linux (x86_64) | `sprite-bootstrap-linux-amd64` |
| Linux (ARM64) | `sprite-bootstrap-linux-arm64` |
| macOS (Intel) | `sprite-bootstrap-darwin-amd64` |
| macOS (Apple Silicon) | `sprite-bootstrap-darwin-arm64` |

```bash
# Example: macOS Apple Silicon
curl -fsSL https://github.com/vaurdan/sprite-bootstrap/releases/latest/download/sprite-bootstrap-darwin-arm64 -o sprite-bootstrap
chmod +x sprite-bootstrap
mv sprite-bootstrap ~/.local/bin/
```

### From Source

```bash
go build -o sprite-bootstrap
```

## Usage

### Start the SSH Server

```bash
sprite-bootstrap serve -l :2222
```

This starts a local SSH server that proxies connections to any sprite. Connect using the sprite name as the SSH username:

```bash
ssh mysprite@localhost -p 2222
```

### IDE-Specific Setup

For IDE-specific configuration and instructions:

```bash
# Zed
sprite-bootstrap zed -s mysprite

# VS Code
sprite-bootstrap vscode -s mysprite

# Open a specific directory
sprite-bootstrap zed -s mysprite --path myproject

# Use a different local port
sprite-bootstrap zed -s mysprite -p 2223
```

These commands configure SSH and provide connection instructions for each IDE.

### Check Status

```bash
sprite-bootstrap status -s mysprite
```

### Stop Proxy

```bash
sprite-bootstrap stop -s mysprite
```

## Architecture

```
Local Machine                              Sprite API
┌────────────────────────┐
│  sprite-bootstrap      │
│  serve -l :2222        │
│                        │    websocket
│  ssh server ───────────│─────────────────▶ sprites API
│                        │                        │
│  ssh mysprite@localhost│                        ▼
│      └── username =    │                   sprite exec
│          sprite name   │
└────────────────────────┘
```

**Benefits over traditional SSH:**
- No sshd installation needed on sprites
- No per-sprite proxy processes
- Single server handles all sprites
- Sprite name = SSH username

## Flags

### Global Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--sprite` | `-s` | Sprite name | (required for zed/vscode) |
| `--org` | `-o` | Organization | (optional) |
| `--port` | `-p` | Local SSH port | 2222 |
| `--path` | | Remote path (relative to /home/sprite or absolute) | /home/sprite |
| `--help` | `-h` | Show help | |

### Serve Command Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--listen` | `-l` | Address to listen on | :2222 |
| `--host-key` | | Path to SSH host key | (auto-generated) |

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

Connect via: ssh %s@localhost -p %d
`, opts.SpriteName, opts.LocalPort)
}
```

That's it - the command is automatically registered.

## Requirements

- Go 1.21+
- `sprite` CLI credentials (run `sprite login` first)

## Acknowledgments

The SSH server implementation is based on [spritessh](https://github.com/jbellerb/spritessh) by jae beller, licensed under the MIT License.

## License

MIT
