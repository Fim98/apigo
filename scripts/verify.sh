#!/usr/bin/env sh
# Full local verification. CI runs the same checks in smaller parallel jobs.
set -eu

cd "$(dirname "$0")/.."

fail() {
  printf '\n✗ %s\n' "$1" >&2
  exit 1
}

printf '→ gofumpt\n'
unformatted=$(go tool gofumpt -l .)
if [ -n "$unformatted" ]; then
  printf '%s\n' "$unformatted"
  fail "Go files need formatting: run 'make fmt'"
fi

printf '→ go vet\n'
go vet ./...

printf '→ mygo vet (native UI lifetimes)\n'
go tool mygo vet .

printf '→ go test (CGO disabled)\n'
CGO_ENABLED=0 go test ./...

printf '→ CLI import boundary\n'
if go list -deps ./cmd/apigo-cli | grep -E '(^apigo/internal/ui$|^apigo/cmd/apigo$|github.com/egoist/mygo/ui)' >/dev/null; then
  fail "cmd/apigo-cli must not import UI packages"
fi

printf '→ cross-compile checks\n'
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...

printf '→ Prettier\n'
npx --no-install prettier --check "**/*.{md,yml,yaml,json,jsonc}"

printf '→ markdownlint\n'
npx --no-install markdownlint-cli2 "**/*.md" "#node_modules" "#build" "#dist" "#.mygo" "#CHANGELOG.md" "#docs/research"

printf '\n✓ verify passed\n'
