#!/usr/bin/env bash
# Build stage and link it into ~/.local/bin, which Ubuntu puts on PATH.
# Rebuilding later updates the linked binary in place.
set -euo pipefail
cd "$(dirname "$0")"
./build.sh
mkdir -p "$HOME/.local/bin"
ln -sf "$PWD/stage" "$HOME/.local/bin/stage"
echo "linked $HOME/.local/bin/stage -> $PWD/stage"
