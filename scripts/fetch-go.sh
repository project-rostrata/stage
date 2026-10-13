#!/usr/bin/env bash
# Fetch the pinned Go toolchain into .toolchain/, verified against toolchain.sha256.
# The toolchain is the only build dependency (PROJECT_BRIEF.md §2). It is never committed.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
dest="$root/.toolchain"
read -r sum file < "$root/toolchain.sha256"
version="${file%.linux-amd64.tar.gz}"

if [[ -x "$dest/go/bin/go" ]] && [[ "$("$dest/go/bin/go" env GOVERSION)" == "$version" ]]; then
  exit 0
fi

mkdir -p "$dest"
tarball="$dest/$file"
if [[ ! -f "$tarball" ]]; then
  echo "fetching $file" >&2
  curl -fsSL -o "$tarball.part" "https://go.dev/dl/$file"
  mv "$tarball.part" "$tarball"
fi

# Verify before unpacking: a tarball that doesn't match the pinned checksum is never used.
if ! echo "$sum  $tarball" | sha256sum -c --quiet -; then
  rm -f "$tarball"
  echo "checksum mismatch for $file; deleted it" >&2
  exit 1
fi

rm -rf "$dest/go"
tar -C "$dest" -xzf "$tarball"
echo "installed $version in $dest/go" >&2
