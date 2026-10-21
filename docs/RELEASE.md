# Release engineering

Status: the CI and tag pipeline are in place for unsigned/ad-hoc builds.
Signing, notarization and auto-update are enabled when the owner provides
credentials (M12).

## 1. Versioning

- Semantic Versioning: `MAJOR.MINOR.PATCH`.
- Pre-1.0: one minor per milestone (`0.4.0` for M3), patch for fixes.
- The version has one source of truth: `mygo.json` `version`.
- `cmd/apigo-cli` gets its version at build time:
  `-ldflags "-X apigo/internal/version.Value=$(version)"`.
- The desktop app reads `mygo.App.Version()` (MyGo links `mygo.json`).
- `CHANGELOG.md` must contain `## [<version>]` for the tag; `scripts/check-release.sh`
  enforces tag/version/changelog agreement.

## 2. Changelog

[Keep a Changelog](https://keepachangelog.com/) format:

```markdown
## [Unreleased]

### Added

- …

## [0.4.0] - 2026-11-20

### Added

- Native workbench: open a collection, edit and send a request.
```

Rules:

- User-visible changes get an entry under `Added`, `Changed`, `Fixed`,
  `Deprecated`, `Removed` or `Security`.
- Internal-only changes may omit the entry.
- The release notes for a tag are extracted from its section with
  `scripts/changelog-section.sh`.
- Update the section heading from `Unreleased` to the version and date as part
  of the release commit.

## 3. Local build

```sh
go tool mygo build                       # this machine
go tool mygo build -platform darwin/universal,windows/amd64,linux/amd64
go tool mygo build -platform web         # native UI in a browser, optional
```

Outputs in `build/<os>-<arch>/`:

| Platform | Artifacts                                                     |
| -------- | ------------------------------------------------------------- |
| macOS    | `apigo.app`, `apigo <version> <arch>.dmg`                     |
| Windows  | `apigo.exe`, `apigo Setup <version> <arch>.exe`               |
| Linux    | `apigo`, `apigo.desktop`, `.deb`, `.tar.gz` with `install.sh` |

The CLI is built separately:

```sh
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X apigo/internal/version.Value=$(node -p "require('./mygo.json').version")" \
  -o dist/apigo-cli ./cmd/apigo-cli
```

Cross-compile with `GOOS`/`GOARCH`; no cgo is required.

## 4. Tag flow

1. Update `mygo.json` version.
2. Move `CHANGELOG.md` `Unreleased` entries into `## [x.y.z] - YYYY-MM-DD`.
3. Run `make verify`, `go tool mygo build` locally, and the conformance suite.
4. Commit: `chore(release): vX.Y.Z`.
5. Tag and push:

   ```sh
   git tag vX.Y.Z
   git push origin main --tags
   ```

6. The release workflow checks the tag, builds every platform, creates a
   **draft** GitHub Release with checksums, and uploads the artifacts.
7. Inspect the draft (installers, sizes, notes), then publish it.

The draft is deliberate: a bad tag must not silently reach users. A future
`publish` job can flip `--draft=false` automatically once signing and smoke
tests are mandatory.

## 5. GitHub Actions

### 5.1 CI (`.github/workflows/ci.yml`)

Runs on pull requests and pushes to `main`:

- docs: Prettier, markdownlint.
- lint: gofumpt, `go vet`, `go tool mygo vet`, golangci-lint.
- test: macOS, Ubuntu, Windows; `CGO_ENABLED=0 go test ./...`.
- race: Ubuntu.
- build: package macOS/Windows/Linux artifacts with `go tool mygo build`;
  upload them as workflow artifacts for inspection.

### 5.2 Release (`.github/workflows/release.yml`)

Triggered by `v*` tags:

- `validate`: checkout, check `v<tag>` equals `mygo.json` version and the
  changelog section exists.
- `build`: matrix:
  - macOS: `darwin/universal` (DMG; signing/notarization on macOS only)
  - Windows: `windows/amd64,windows/arm64` (Setup EXE on Ubuntu with NSIS)
  - Linux: `linux/amd64,linux/arm64` (deb + tar.gz)
- `release`: download all artifacts, generate `SHA256SUMS.txt`, create a draft
  release with notes from `CHANGELOG.md` plus generated notes, upload the
  installers and checksums.

The workflow uses `go tool mygo build`, not a globally installed CLI, so the
MyGo version is pinned by `go.mod`.

## 6. Signing and notarization

These are optional until the owner provides credentials. A build without them is
ad-hoc signed (macOS) or unsigned (Windows) and must be labelled as such.

### macOS

Secrets:

| Secret                       | Contents                                      |
| ---------------------------- | --------------------------------------------- |
| `MACOS_CERTIFICATE`          | base64 of the Developer ID Application `.p12` |
| `MACOS_CERTIFICATE_PASSWORD` | `.p12` password                               |
| `APPLE_ID`                   | Apple ID                                      |
| `APPLE_TEAM_ID`              | Team ID                                       |
| `APPLE_APP_PASSWORD`         | app-specific password                         |

Workflow: import the certificate into a temporary keychain, store notary
credentials with `notarytool`, set `NOTARY_KEYCHAIN`, and configure
`mygo.json`:

```json
{
  "macos": {
    "signingIdentity": "Developer ID Application: Name (TEAMID)",
    "notarize": { "keychainProfile": "notary" }
  }
}
```

`mygo build` signs the app, submits the DMG, waits and staples the ticket.

### Windows

Secrets: `WINDOWS_CERTIFICATE` (base64 `.pfx`), `WINDOWS_CERTIFICATE_PASSWORD`.
Install `osslsigncode` and `nsis` on the Ubuntu runner; point `mygo.json`
`windows.certificate` at the decoded file. Hardware-token/cloud signing uses
`windows.signCommand` on a Windows runner instead.

### Auto-update (M12)

1. `go tool mygo keygen`; keep the private key offline.
2. Add the public key to `mygo.json` `updates.publicKey`, plus
   `updates.github: "owner/apigo"`.
3. Store the private key as `MYGO_UPDATER_PRIVATE_KEY` in GitHub secrets.
4. Build with `-upload` so update manifests are uploaded to the draft.
5. Publish; apps check the signed manifest and verify the signature.

Do not enable updates before step 1–2 are complete; an unsigned update path is
worse than no updates.

## 7. Package managers (M12)

| Channel                     | Artifact                                          |
| --------------------------- | ------------------------------------------------- |
| Homebrew cask               | DMG                                               |
| winget / Scoop / Chocolatey | Setup EXE                                         |
| apt / rpm                   | deb / rpm (rpm needs an additional package step)  |
| Snap / Flatpak              | Linux builds                                      |
| Docker (CLI only)           | `apigo-cli` image, alpine and debian, amd64/arm64 |

Package-manager manifests live in the release repository, not in this one,
unless a tap/bucket is created.

## 8. Exit codes

Defined in [REQUEST_ENGINE.md](REQUEST_ENGINE.md#15-exit-codes-cli) and matched
by `cmd/apigo-cli` tests. Do not change them without a major version.

## 9. Release checklist

- [ ] `make verify` passes on a clean checkout.
- [ ] `go tool mygo build` succeeds for all target platforms.
- [ ] Parser conformance and capability matrix audit pass for the milestone.
- [ ] `CHANGELOG.md` section exists and is accurate.
- [ ] `scripts/check-release.sh vX.Y.Z` passes.
- [ ] Draft release artifacts have expected names and sizes.
- [ ] macOS DMG opens on a clean machine (Gatekeeper result recorded).
- [ ] Windows installer runs on a clean machine (SmartScreen result recorded).
- [ ] Linux deb/tar.gz install/uninstall on a clean container.
- [ ] CLI binary runs `apigo-cli version` and a sample collection.
- [ ] SHA256SUMS and SBOM attached.
- [ ] Update signature verified (when updates are enabled).
- [ ] Release notes published; docs version references updated.

## 10. Rollback

- Yank the GitHub Release (mark as pre-release or delete it) and publish a fixed
  patch version; never rewrite a published tag.
- For auto-updates, publish the fixed version with a higher semver; the updater
  only moves forward.
- Document the incident in `CHANGELOG.md` under `Security` or `Fixed`.
