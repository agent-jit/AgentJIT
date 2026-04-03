#!/bin/sh
set -eu

REPO="agent-jit/AgentJIT"
BINARY="aj"
INSTALL_DIR="/usr/local/bin"

main() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
    esac

    case "$os" in
        linux|darwin) ;;
        *) echo "Unsupported OS: $os. Use the Windows zip from GitHub Releases." >&2; exit 1 ;;
    esac

    # Get latest version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d '"' -f4)
    if [ -z "$version" ]; then
        echo "Error: could not determine latest version" >&2
        exit 1
    fi
    version_num=$(echo "$version" | tr -d 'v')

    echo "Installing ${BINARY} ${version} (${os}/${arch})..."

    url="https://github.com/${REPO}/releases/download/${version}/${BINARY}_${version_num}_${os}_${arch}.tar.gz"

    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    curl -fsSL "$url" -o "${tmpdir}/${BINARY}.tar.gz"
    tar xzf "${tmpdir}/${BINARY}.tar.gz" -C "$tmpdir" "$BINARY"

    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        echo "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi

    chmod +x "${INSTALL_DIR}/${BINARY}"

    echo "Installed ${BINARY} ${version} to ${INSTALL_DIR}/${BINARY}"
    echo ""
    echo "Next steps:"
    echo "  aj init       # Set up hooks and config"
    echo "  aj --help     # See all commands"
}

main
