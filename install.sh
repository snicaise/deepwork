#!/usr/bin/env bash
# deepwork installer — curl -fsSL https://raw.githubusercontent.com/sebastien/deepwork/main/install.sh | bash
set -euo pipefail

REPO="${DEEPWORK_REPO:-sebastien/deepwork}"
VERSION="${DEEPWORK_VERSION:-latest}"

if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "deepwork is macOS-only (detected: $(uname -s))" >&2
    exit 1
fi

arch="$(uname -m)"
case "$arch" in
    arm64|aarch64) arch="arm64" ;;
    x86_64|amd64) arch="amd64" ;;
    *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

if [[ "$VERSION" == "latest" ]]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
        | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)"
    if [[ -z "$VERSION" ]]; then
        echo "could not resolve latest version from GitHub" >&2
        exit 1
    fi
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

version_noprefix="${VERSION#v}"
archive="deepwork_${version_noprefix}_darwin_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$VERSION"

echo "→ downloading deepwork $VERSION ($arch)"
curl -fsSL "$base_url/$archive" -o "$tmpdir/$archive"
curl -fsSL "$base_url/checksums.txt" -o "$tmpdir/checksums.txt"

echo "→ verifying checksum"
(cd "$tmpdir" && shasum -a 256 -c --ignore-missing checksums.txt >/dev/null)

echo "→ extracting"
tar -xzf "$tmpdir/$archive" -C "$tmpdir"

xattr -d com.apple.quarantine "$tmpdir/deepwork" 2>/dev/null || true
xattr -d com.apple.quarantine "$tmpdir/deepwork-apply" 2>/dev/null || true

echo "→ running sudo deepwork install"
sudo "$tmpdir/deepwork" install

cat <<EOF

✓ deepwork $VERSION installed

Next steps:
  deepwork edit      # customize ~/.deepwork/config.yml
  deepwork status    # see current state
  deepwork doctor    # check for DoH bypasses in your browsers
EOF
