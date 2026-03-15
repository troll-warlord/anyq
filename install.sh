#!/bin/sh
# Install anyq — the unified JSON/YAML/TOML query tool
# Usage: curl -sSfL https://raw.githubusercontent.com/troll-warlord/anyq/main/install.sh | sh
set -e

REPO="troll-warlord/anyq"
BINARY="anyq"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# Fetch latest release tag
LATEST=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" \
         | grep '"tag_name"' \
         | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$LATEST" ]; then
  echo "Could not determine latest release. Check https://github.com/${REPO}/releases" >&2
  exit 1
fi

VERSION="${LATEST#v}"
TARBALL="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${TARBALL}"

echo "Installing ${BINARY} ${LATEST} (${OS}/${ARCH}) to ${INSTALL_DIR} ..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -sSfL "$URL" -o "${TMP}/${TARBALL}"
tar -xzf "${TMP}/${TARBALL}" -C "$TMP" "$BINARY"
chmod +x "${TMP}/${BINARY}"

# Install (use sudo if needed)
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  sudo mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo "Done! Run: ${BINARY} --version"
