#!/usr/bin/env sh
# Print the CHANGELOG.md section for a version, without its heading.
# Usage: scripts/changelog-section.sh 0.1.0
set -eu

cd "$(dirname "$0")/.."

version=${1:-}
if [ -z "$version" ]; then
  printf 'usage: %s <version>\n' "$0" >&2
  exit 2
fi

awk -v version="$version" '
  $0 ~ "^## \\[" version "\\]" { found = 1; next }
  found && /^## \[/ { exit }
  found { print }
' CHANGELOG.md
