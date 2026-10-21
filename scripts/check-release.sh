#!/usr/bin/env sh
# Check that the pushed tag, mygo.json version and CHANGELOG agree.
set -eu

cd "$(dirname "$0")/.."

version=$(jq -r .version mygo.json)
tag=${1:-${GITHUB_REF_NAME:-}}

if [ -z "$tag" ]; then
  tag=$(git describe --tags --exact-match 2>/dev/null || true)
fi

if [ -z "$tag" ]; then
  printf 'usage: %s <vX.Y.Z> (or run on a tag)\n' "$0" >&2
  exit 2
fi

if [ "$tag" != "v$version" ]; then
  printf '✗ tag %s does not match mygo.json version v%s\n' "$tag" "$version" >&2
  exit 1
fi

if ! grep -q "^## \[$version\]" CHANGELOG.md; then
  printf '✗ CHANGELOG.md has no section for %s\n' "$version" >&2
  exit 1
fi

printf '✓ %s matches mygo.json and CHANGELOG.md\n' "$tag"
