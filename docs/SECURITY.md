# Security

apigo handles API credentials, scripts and collections that may come from
untrusted repositories. This document defines the security model and the rules
that keep it true.

## 1. Threat model

| Asset                                 | Threat                                         | Mitigation                                                                              |
| ------------------------------------- | ---------------------------------------------- | --------------------------------------------------------------------------------------- |
| API tokens, passwords, client secrets | Leak through logs, exports, Git, crash reports | Keychain storage, UI masking, redaction, no telemetry, secret-leak warning              |
| Collection files                      | Malicious `.bru` content exploits the parser   | Memory-bounded parser, fuzz tests, no code execution from files except explicit scripts |
| User scripts                          | Arbitrary code execution / data exfiltration   | goja sandbox, whitelist modules, no fs/net/process, timeout and output caps             |
| TLS traffic                           | MITM                                           | Verification on by default; insecure mode explicit and per-request                      |
| Git operations                        | Credential theft, destructive commands         | Delegate credentials to Git; no custom credential storage; confirm destructive actions  |
| Updates                               | Supply-chain compromise                        | Signed update manifests; HTTPS; key stored offline                                      |
| Imported files                        | Traversal, huge files, zip bombs               | Size limits, path validation, no automatic extraction outside a chosen folder           |
| HTML reports                          | Script injection in report                     | Escape all user data; no inline script from request content                             |
| Dependencies                          | Vulnerable transitive package                  | Minimal dependencies, `govulncheck`, Dependabot, review                                 |

The app is an API client: it intentionally makes requests to user-supplied URLs,
including localhost and private networks. That is not SSRF in this context, but
the UI must show the exact destination and never auto-send to a URL the user did
not confirm.

## 2. Secrets

- Secret environment variables are stored in the OS keychain (MyGo secrets),
  never in `.bru`, `bruno.json`, history, logs or exports. Values over MyGo's
  2,560-byte keychain limit are encrypted (AES-GCM) with a key held in the
  keychain and stored as a `0600` file; the plaintext never touches disk.
- The `.bru` environment file contains only secret **names** (`vars:secret`).
- The UI masks values by default; "reveal" is explicit and temporary.
- Logs, timelines, reports and error messages redact `Authorization`,
  `Proxy-Authorization`, `Cookie`, `Set-Cookie`, API-key headers and configured
  secret names.
- Exported collections drop secret values and print a warning.
- Git integration warns before committing a file that looks like it contains a
  secret (long random token, private key block, `password:`).
- Test fixtures use obvious dummy values; CI fails if a known real-looking token
  pattern is committed.

## 3. Script sandbox

- goja only; no cgo, no `os/exec`, no filesystem, no network.
- `require` is a whitelist; local modules are restricted to the collection root
  and cannot escape via `..` or symlinks.
- Timeout (default 5s), output cap (1 MiB), input size cap (1 MiB per script).
- A script cannot mutate global process state or other tabs' data.
- The first run of a collection containing scripts shows a warning; the user can
  remember the decision per collection.
- Unsupported Node APIs fail with an explicit error, never a silent stub that
  returns wrong data.

Scripts are still code the user chose to run. The sandbox reduces blast radius;
it is not a guarantee against logic bugs or data exfiltration through the
request itself.

## 4. TLS and certificates

- Verification on by default, including hostname checks.
- `--insecure` / per-request "skip verification" is visibly marked in the UI and
  in reports; it is never persisted as a global default without confirmation.
- Custom CA bundles extend system roots; `--ignore-truststore` is explicit.
- Client certificates and passphrases are stored in the keychain or referenced
  by path; passphrases are never logged.
- TLS 1.0/1.1 are not forced; the system defaults apply. A future minimum-version
  setting needs an ADR.

## 5. Files and paths

- All collection paths are cleaned and checked to stay within the collection
  root for operations initiated by a collection (file bodies, local modules,
  exports).
- Import file size is capped (default 50 MiB); archives are not auto-extracted.
- Temporary response bodies live in the app cache directory with restrictive
  permissions and are deleted on quit/age.
- Symlinks are resolved and rejected if they escape the collection root.
- Atomic writes (temp file + rename) prevent partial `.bru` files.

## 6. Git

- Credentials are handled by the system `git` and its credential helpers; apigo
  never stores Git passwords.
- Destructive actions (discard, reset, force push) require confirmation.
- The app does not run arbitrary hooks with elevated privileges; hooks run as
  Git normally does.
- Commit messages and diffs are shown before commit.

## 7. Updates and distribution

- Auto-update is disabled until signed manifests are configured (M12).
- Update public keys are embedded; private keys never enter the repository or
  CI logs.
- macOS builds are signed with a Developer ID and notarized; Windows builds are
  Authenticode-signed before being advertised as trusted.
- Release artifacts include SHA-256 checksums and an SBOM.

## 8. Dependencies

- Prefer stdlib. New dependencies need a reason, a license check and a
  `CGO_ENABLED=0` build.
- `govulncheck ./...` runs in CI (allowlist only with a documented reason).
- Dependabot updates Go modules, GitHub Actions and npm dev tooling weekly.
- No dependency may phone home at runtime; audit new packages for telemetry.

## 9. Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Contact the project
owner privately with reproduction steps and affected versions. Until a security
policy/contact is published, use the repository's private security advisory
feature once the GitHub remote exists.

## 10. Security checklist for PRs

- [ ] No secrets, tokens or real credentials in code, tests or fixtures.
- [ ] User-controlled paths are validated; file operations are bounded.
- [ ] New network calls are user-initiated and documented.
- [ ] Logs/reports redact sensitive headers and variables.
- [ ] Script-visible APIs do not widen the sandbox.
- [ ] Dependencies pass license and vulnerability review.
