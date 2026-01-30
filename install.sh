#!/bin/sh
# Sprite Bootstrap Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/vaurdan/sprite-bootstrap/main/install.sh | sh

set -e

REPO="vaurdan/sprite-bootstrap"
BINARY_NAME="sprite-bootstrap"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info() {
    printf "${CYAN}%s${NC}\n" "$1"
}

success() {
    printf "${GREEN}%s${NC}\n" "$1"
}

warn() {
    printf "${YELLOW}%s${NC}\n" "$1"
}

error() {
    printf "${RED}Error: %s${NC}\n" "$1" >&2
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Get latest release version
get_latest_version() {
    LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"

    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -fsSL "$LATEST_URL" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "$LATEST_URL" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        error "curl or wget is required"
    fi

    if [ -z "$VERSION" ]; then
        error "Failed to get latest version"
    fi

    info "Latest version: $VERSION"
}

# Download and install
install() {
    BINARY_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${PLATFORM}"

    info "Downloading from: $BINARY_URL"

    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"

    # Download binary
    TMP_FILE=$(mktemp)
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$BINARY_URL" -o "$TMP_FILE" || error "Download failed"
    else
        wget -q "$BINARY_URL" -O "$TMP_FILE" || error "Download failed"
    fi

    # Install binary
    chmod +x "$TMP_FILE"
    mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY_NAME}"

    success "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
}

# Check if install directory is in PATH
check_path() {
    case ":$PATH:" in
        *":$INSTALL_DIR:"*)
            ;;
        *)
            warn ""
            warn "Note: $INSTALL_DIR is not in your PATH."
            warn "Add it to your shell profile:"
            warn ""
            warn "  export PATH=\"\$PATH:$INSTALL_DIR\""
            warn ""
            ;;
    esac
}

# Verify installation
verify() {
    if [ -x "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        success ""
        success "Installation complete!"
        info "Run '${BINARY_NAME} --help' to get started."
    else
        error "Installation verification failed"
    fi
}

main() {
    info "Installing ${BINARY_NAME}..."
    info ""

    detect_platform
    get_latest_version
    install
    check_path
    verify
}

main
