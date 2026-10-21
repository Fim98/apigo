# Request engine

Status: specification for the behavior implemented by `internal/vars`,
`internal/httpx`, `internal/auth`, `internal/assert` and `internal/runner`.
It follows Bruno 2.0.0's execution semantics, with the divergences listed in
§13.

## 1. Pipeline

For one HTTP request:

```text
load request + folder + collection + environment
  → merge headers/auth/vars/scripts (collection → folder → request)
  → collect prompt variables ({{?Name}})
  → run pre-request script (may mutate vars/headers/body)
  → resolve variables and interpolate every field
  → build URL, headers, body
  → apply auth provider
  → apply TLS/proxy/cookie settings
  → send with context + timeout
  → capture response (stream or buffer)
  → store cookies
  → run post-response vars expressions
  → run post-response script
  → run tests and assertions
  → persist runtime/env mutations (desktop: ask/auto; CLI: only when configured)
  → return Result + Timeline
```

Rules that are easy to get wrong:

- Pre-request scripts run **before** final interpolation, so a script can set a
  variable used by the URL/body.
- Auth is applied **after** user headers, so it can set or replace
  `Authorization`.
- Post-response vars run before the post-response script and tests.
- A transport failure still produces a result with the error and timeline; a
  non-2xx response is not an error.
- Post-response scripts and tests run for non-2xx responses, and for SSE after
  the stream closes.
- `content-type: false` is an internal sentinel for "no body, suppress axios's
  default content type". It must never be sent or shown to scripts as a real
  header (except AWS SigV4, which needs a content type for signing).

## 2. Variable scopes and precedence

Lowest to highest:

1. `process.env.*` (addressed explicitly, not merged into bare names)
2. global environment (workspace-scoped)
3. collection variables (`collection.bru` `vars:pre-request`)
4. selected environment (including `extends`), then `--env-var` overrides
5. folder variables (nearest folder first, then ancestors)
6. request variables (`vars:pre-request`)
7. OAuth2 credential variables (`$oauth2.<credentialsId>.<key>`)
8. runtime variables (`bru.setVar`)
9. prompt variable answers (`{{?Name}}`)

Important details:

- `process.env` is a separate namespace: `{{process.env.FOO}}` resolves from the
  OS environment overlaid with the collection `.env`.
- Environment secrets win over non-secret variables of the same name when
  `extends` is resolved.
- A whole-string reference (`"{{count}}"`) returns the referenced value with its
  type; interpolation inside a larger string returns a string.
- Typed variables (`@number`, `@boolean`, `@object`) are coerced by
  `parseValueByDataType`; invalid coercions fall back to the raw string.
- Unresolved variables are recorded and surfaced in the UI; they do not fail the
  request. The literal `{{name}}` stays in the URL/body, matching Bruno.
- Recursive interpolation is bounded (cycle guard + max depth 10). A cycle keeps
  the last resolved text and reports a warning.

## 3. Interpolation

Grammar: `{{ ... }}` with dot paths (`user.address.city`), array indexes
(`items[0]`), and the special forms `{{process.env.NAME}}` and `{{?Prompt}}`.

Algorithm:

1. Replace dynamic variables (`{{$name}}`) first, using a per-call memo so the
   same token repeated in one string yields the same value.
2. Look up the remaining token through the scopes above (dot path lookup).
3. Recursively interpolate the result while matches remain; guard cycles.
4. Apply JSON escaping when interpolating into a JSON string context.

Dynamic variables (Bruno's faker set, ~120 names) are grouped as: ids/time
(`$guid`, `$timestamp`, `$isoTimestamp`, `$randomUUID`, `$randomNanoId`,
`$randomInt`, `$randomBoolean`, `$randomSemver`, ...), internet, person,
location, finance, company, commerce, date, file/system, text and misc. The Go
implementation provides the complete list from Bruno 2.0.0's
`faker-functions.ts`; see `internal/vars/dynamic.go` for the authoritative
table.

Prompt variables `{{?Prompt text}}` are collected from the prepared request,
all variable scopes and proxy/cert config. The desktop app prompts before send;
the CLI and runner skip such requests with `status: skipped` and a reason.

## 4. Authentication

Mode list: `inherit`, `none`, `basic`, `bearer`, `apikey`, `digest`, `ntlm`,
`oauth1`, `oauth2`, `awsv4`, `wsse`, `akamai-edgegrid`.

| Mode              | Behavior                                                                                                                                                                                                                                                              |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `basic`           | `Authorization: Basic base64(username:password)`, built after interpolation                                                                                                                                                                                           |
| `bearer`          | `Authorization: Bearer <token>` (prefix verbatim)                                                                                                                                                                                                                     |
| `apikey`          | `placement: header` → `<apiKeyHeaderName>: <value>`; `placement: queryparams` → `URL.searchParams.set(key,value)`; empty key is a no-op                                                                                                                               |
| `digest`          | Challenge-response on 401. Bruno accepts **MD5 only**; `HA1=MD5(user:realm:pass)`, `HA2=MD5(method:uri)`, `qop=auth` → `MD5(HA1:nonce:nc:cnonce:auth:HA2)` with `nc=00000001`, random cnonce, optional `opaque`; one retry; skipped if `Authorization` already exists |
| `ntlm`            | NTLMv2 via a RoundTripper (domain/username/password); redirects drop the auth and renegotiate                                                                                                                                                                         |
| `oauth1`          | RFC 5849: HMAC-SHA1/SHA256/SHA512, RSA-SHA1/SHA256/SHA512, PLAINTEXT; body hash per draft-eaton (not for form bodies); placement header/query/body; `realm`, callback, verifier, timestamp/nonce overrides                                                            |
| `oauth2`          | Four grants: authorization code (+ PKCE S256), implicit, client credentials, password. Token placement header/query; token source access/id; additional params per phase; cache/refresh                                                                               |
| `awsv4`           | AWS Signature V4 with region/service/credentials/session token, or a named profile from `~/.aws`; cross-origin redirects are not re-signed                                                                                                                            |
| `wsse`            | `X-WSSE: UsernameToken Username="…", PasswordDigest="…", Nonce="…", Created="…"`; **Bruno's digest quirk is reproduced by default** (see §13)                                                                                                                         |
| `akamai-edgegrid` | `EG1-HMAC-SHA256` with client token/access token/timestamp/nonce; canonical signed headers, content hash and truncated-body rule                                                                                                                                      |
| `inherit`         | Nearest folder auth, else collection auth; `none` overrides                                                                                                                                                                                                           |

OAuth2 details:

- Authorization URL includes `response_type`, `client_id`, `redirect_uri`,
  `scope`, `state`; PKCE adds `code_challenge` and
  `code_challenge_method=S256` (verifier = 44 hex chars).
- User-supplied `state` is used verbatim; otherwise 16 random bytes are
  generated and validated on callback.
- Client credentials and password support `credentials_placement`:
  `basic_auth_header` or body.
- Token expiry: `created_at + expires_in*1000 < now`; no `expires_in` means
  never expires. If expired: refresh when `auto_refresh_token` and a refresh
  token exist, else fetch when `auto_fetch_token`, else keep the expired token.
- Token cache key: collection + token URL + credentials ID (desktop persists,
  CLI in-memory).
- Token variables are exposed to scripts as
  `$oauth2.<credentialsId>.<key>`; `bru.resetOauth2Credential(id)` clears them.
- Interactive grants open the system browser (or an app window) and use a
  loopback callback. The CLI supports the non-interactive grants; interactive
  grants require a browser and are documented per command.

## 5. URL building

- Base URL may omit the scheme; `http://` is prepended (Bruno behavior).
- Path params replace `:name` segments (including OData `Func(:name)`), after
  interpolation and URL encoding rules.
- Query params are appended in order; duplicates are preserved; disabled params
  are omitted; values are encoded once.
- `settings.encodeUrl` controls whether the final URL is percent-encoded as a
  whole. The default matches Bruno's `encodeUrl: true` behavior.
- Fragments are preserved if present; they are never sent on the wire.
- Invalid URLs produce a `KindConfig` error before any network I/O.

## 6. Bodies

| Mode             | Bytes                                                                                    | Default Content-Type                            |
| ---------------- | ---------------------------------------------------------------------------------------- | ----------------------------------------------- |
| `none`           | empty                                                                                    | suppressed (`content-type: false`)              |
| `json`           | comment-stripped JSON, then interpolated as JSON                                         | `application/json`                              |
| `text`           | raw                                                                                      | `text/plain`                                    |
| `xml`            | raw                                                                                      | `application/xml`                               |
| `sparql`         | raw                                                                                      | `application/sparql-query`                      |
| `file`           | one or more files; stream above 20 MiB                                                   | `application/octet-stream`, overridden per file |
| `formUrlEncoded` | enabled rows, repeated keys for arrays                                                   | `application/x-www-form-urlencoded`             |
| `multipartForm`  | enabled rows; text/file parts with optional per-part content type; user boundary honored | `multipart/form-data`                           |
| `graphql`        | JSON `{query, variables}`; variables string is interpolated then parsed                  | `application/json`                              |
| `grpc` / `ws`    | protocol-specific message lists                                                          | protocol-specific                               |

- Multipart bodies must be replayable on 307/308; retain the original part
  list, not a consumed stream.
- File paths are resolved relative to the collection root; missing files are a
  `KindIO` error with the path.
- JSON comment stripping only applies to `body:json` files (Bruno's
  `decomment` behavior); if stripping makes the JSON invalid, send the raw
  body.

## 7. Assertions and tests

### 7.1 Assertion DSL

Rows are `lhs: <operator> <operand>`; default operator is `eq`. The left side is
evaluated against the response context; the right side is interpolated and
evaluated with literal typing.

Operators: `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `notIn`, `contains`,
`notContains`, `length`, `matches`, `notMatches`, `startsWith`, `endsWith`,
`between`, `isEmpty`, `isNotEmpty`, `isNull`, `isUndefined`, `isDefined`,
`isTruthy`, `isFalsy`, `isJson`, `isNumber`, `isString`, `isBoolean`,
`isArray`, plus `jsonSchema`/`jsonBody` where Bruno supports them.

Evaluation context: variable scopes flattened, plus `bru`, `req`, `res`.
Process env entries are bare globals in Bruno's safe sandbox; apigo matches
that and also exposes `process.env` for developer-mode scripts.

### 7.2 `test()` and `expect`

- `test(description, fn)` is async; failures record `message`, and for chai
  assertions also `actual`, `expected`, `stack`.
- A failing test does not abort the script; all test promises are awaited.
- Tests run in pre-request, post-response and the `tests` block; results are
  reported per phase.
- `expect` supports the chai 4 surface plus `chai-string`, `json`, `jsonSchema`
  (draft-07), `match` and `jsonBody`; unsupported matchers fail clearly.

## 8. Response model and timing

```go
type Response struct {
    Status     int
    StatusText string
    Headers    http.Header
    Cookies    []*http.Cookie
    Body       []byte     // inline when <= inlineBodyLimit
    BodyFile   string     // temp file for large bodies
    Size       BodySize   // header/body/total
    URL        string
    Redirects  []Redirect
    Timeline   []TimelineEntry
    Duration   time.Duration
}
```

Timeline entries: preparation, request headers, request body, response headers,
response body, redirects, errors. Timings: DNS, connect, TLS handshake, TTFB,
download, total. Values that the transport cannot measure (connection reuse)
are zero, not fabricated.

`res.getSize()` reports header bytes + body bytes; `res.getBody()` returns the
parsed body (JSON object when the content type is JSON and parsing is enabled);
`res.getDataBuffer()` returns raw bytes.

## 9. Cookies

- One jar per collection, shared by requests, scripts and the cookie UI.
- Desktop persists the jar (encrypted at rest where possible); CLI keeps it in
  memory and clears it at exit unless a future flag says otherwise.
- `settings.shouldSendCookies` / `shouldStoreCookies` (desktop preferences) are
  honored; the CLI has `--disable-cookies`.
- An explicit `Cookie` header is merged with the jar; jar values win on name
  collision, then the header is re-serialized.
- `__Host-`/`__Secure-` prefix rules and expiry are enforced by the jar, not
  ad hoc parsing.

## 10. TLS, proxies, DNS

- TLS verification is on by default. `--insecure` and the per-request setting
  are explicit. Custom CA bundles extend, never replace, the system roots unless
  `--ignore-truststore` is set.
- Client certificates: collection/global cert list, matched by domain glob;
  PEM (cert+key) and PKCS#12 with passphrase. First match wins.
- Proxy modes: off, manual HTTP/HTTPS, SOCKS4/SOCKS5, system, PAC. MyGo has no
  system-proxy API, so "system" is implemented per OS (macOS `scutil --proxy`,
  Windows registry/WinHTTP, Linux env + GNOME `gsettings`) and PAC is evaluated
  with goja. Bypass lists split on comma/semicolon/whitespace, support domain
  suffixes and `host:port`. Proxy auth is supported and redacted from logs.
- PAC evaluation uses goja to run `FindProxyForURL`; the result maps to the
  corresponding transport. System-proxy discovery is best-effort and never
  overrides an explicit request setting.
- DNS: the system resolver. `localhost` variants are normalized to avoid the
  IPv6/IPv4 surprise Bruno works around. No DNS-over-HTTPS.
- Default request headers: `User-Agent: apigo/<version>` (Bruno sends
  `bruno-runtime/<version>`; documented divergence), `Accept: */*`,
  `Accept-Encoding: gzip, deflate, br`. `Host` and `Connection` are controlled
  by the transport; `settings.omitHeaders` can suppress defaults except `Host`.
  `Connection` omission requires disabling keep-alive.

## 11. Redirects and header rules

- Default `maxRedirects = 5`; `followRedirects: false` means 0.
- 301/302/303 rewrite to GET and drop body/content-length/content-type except
  for HEAD.
- 307/308 preserve method and body; multipart is replayed from the retained
  parts.
- Cross-origin: strip `Authorization`/`Proxy-Authorization` unless
  `forwardAuthorizationHeader` (default true); always strip `x-amz-*` and
  disable SigV4 re-signing.
- Relative `Location` resolves against the current URL; cookies are re-read for
  the new URL.
- Sensitive headers are redacted in the timeline and reports unless the user
  explicitly disables redaction.

## 12. Cancellation, timeouts and limits

- Every request takes a context. Closing a tab, cancelling a run, quitting the
  app or `--bail` cancels in-flight work.
- Default timeout: none (`0`), matching Bruno. The UI shows a warning and
  suggests a timeout; the CLI accepts `--timeout` as an extension.
- Response bodies larger than `inlineBodyLimit` (10 MiB) stream to a temp file;
  the UI virtualizes the view. The runner has its own total response cap
  (default 100 MiB) and reports truncation.
- Scripts have a 5s default timeout and 1 MiB output cap.
- Redirect loops, recursive variables and `setNextRequest` loops are bounded
  (5000 redirects is not reachable due to max 5; variable depth 10; runner jumps
  10000).

## 13. Deliberate divergences from Bruno

Each divergence is a matrix row and, where behavioral, has a test.

| Topic             | Bruno 2.0.0                             | apigo                                                                            | Why                                                             |
| ----------------- | --------------------------------------- | -------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| HTTP version      | HTTP/1.1 only                           | HTTP/1.1 by default; HTTP/2 opt-in per request                                   | Parity by default; h2 is a useful extension                     |
| Large responses   | fully buffered                          | stream to disk above 10 MiB                                                      | Memory safety; response bytes identical                         |
| Downloads         | buffered                                | stream to file, same model                                                       | Memory safety                                                   |
| User-Agent        | `bruno-runtime/<version>`               | `apigo/<version>`                                                                | Honest identity; servers keying on UA are rare and can override |
| WSSE digest       | base64(hex(sha1(...)))                  | same by default, with a strict option                                            | Reproduce Bruno's wire behavior; strict mode for OASIS servers  |
| OAuth2 CLI        | interactive grants send unauthenticated | supports non-interactive grants; interactive requires browser and errors clearly | Do not silently send unauthenticated requests                   |
| Scripts           | QuickJS safe + Node developer mode      | goja safe mode only; no `fs`/npm                                                 | Pure Go; documented in SCRIPTING.md                             |
| Runner data files | absent                                  | not implemented in 1.0 (possible extension)                                      | No parity target                                                |
| Parallel/retries  | absent                                  | not implemented                                                                  | No parity target                                                |
| gRPC/WS in runner | skipped                                 | supported where the engine can, otherwise explicit skip                          | Superset; conformance can skip                                  |
| MCP               | absent                                  | not implemented                                                                  | Not in target revision                                          |
| AI                | present (chat/autocomplete)             | opt-in M10, only with owner approval                                             | Privacy default                                                 |

## 14. Conformance

`internal/httpx/testdata/` contains a table of cases: method, URL, headers,
body, settings, expected request line/headers/body (normalized) and expected
response. Integration tests use `httptest` servers for redirects, cookies,
multipart, gzip/br, TLS, proxy, NTLM and digest.

The Bruno testbench collection (`packages/bruno-tests/collection`) is the
reference for end-to-end cases; the runner conformance harness can execute both
CLIs against the same collection and compare normalized results.

## 15. Exit codes (CLI)

Match Bruno's contract:

| Code | Meaning                           |
| ---- | --------------------------------- |
| 0    | success                           |
| 1    | failed request, assertion or test |
| 2    | output directory missing          |
| 3    | request chain exceeded jump limit |
| 4    | not inside a collection           |
| 5    | path/file not found               |
| 6    | environment not found             |
| 7    | env override not a string/array   |
| 8    | env override not `name=value`     |
| 9    | invalid output format             |
| 10   | invalid env/client-cert file      |
| 11   | workspace not found               |
| 12   | global env requires a workspace   |
| 13   | global environment not found      |
| 255  | unhandled/generic error           |
