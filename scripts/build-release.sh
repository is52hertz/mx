#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p dist
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for os in darwin linux; do
    for arch in amd64 arm64; do
        CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$tmp/mx" .
        tar -czf "dist/mx_${os}_${arch}.tar.gz" -C "$tmp" mx
    done
done
if command -v sha256sum >/dev/null 2>&1; then
    (cd dist && sha256sum mx_*.tar.gz > SHA256SUMS)
else
    (cd dist && shasum -a 256 mx_*.tar.gz > SHA256SUMS)
fi
