#!/bin/bash

# lazysync installer
# Usage:
#   curl https://raw.githubusercontent.com/Clumsyvortex123/lazy-sync-scp/main/scripts/install_update_linux.sh | bash
#
# Set DIR to change install location:
#   DIR="$HOME/.local/bin" curl ... | bash

set -e

DIR="${DIR:-"/usr/local/bin"}"

# map architecture
ARCH=$(uname -m)
case $ARCH in
    i386|i686) ARCH=x86 ;;
    x86_64) ARCH=x86_64 ;;
    armv6*) ARCH=armv6 ;;
    armv7*) ARCH=armv7 ;;
    aarch64*|arm64) ARCH=arm64 ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

OS=$(uname -s)
case $OS in
    Linux) OS=Linux ;;
    Darwin) OS=Darwin ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

GITHUB_REPO="Clumsyvortex123/lazy-sync-scp"

echo "Fetching latest release..."
GITHUB_LATEST_VERSION=$(curl -L -s -H 'Accept: application/json' "https://github.com/${GITHUB_REPO}/releases/latest" | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')

if [ -z "$GITHUB_LATEST_VERSION" ] || [ "$GITHUB_LATEST_VERSION" = "null" ]; then
    echo "Error: Could not determine latest version. Check https://github.com/${GITHUB_REPO}/releases"
    exit 1
fi

echo "Latest version: $GITHUB_LATEST_VERSION"

GITHUB_FILE="lazysync_${GITHUB_LATEST_VERSION#v}_${OS}_${ARCH}.tar.gz"
GITHUB_URL="https://github.com/${GITHUB_REPO}/releases/download/${GITHUB_LATEST_VERSION}/${GITHUB_FILE}"

echo "Downloading ${GITHUB_FILE}..."
curl -L -o lazysync.tar.gz "$GITHUB_URL"

echo "Extracting..."
tar xzf lazysync.tar.gz lazysync

echo "Installing to ${DIR}..."
if [ -w "$DIR" ]; then
    install -Dm 755 lazysync -t "$DIR"
else
    echo "(requires sudo)"
    sudo install -Dm 755 lazysync -t "$DIR"
fi

rm lazysync lazysync.tar.gz

echo ""
echo "lazysync ${GITHUB_LATEST_VERSION} installed to ${DIR}/lazysync"
echo "Run 'lazysync' from anywhere to start."
