# ADR-0004: `net/http` for request execution

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

The request engine must support HTTP/1.1, redirects, proxies, TLS client
certificates, custom CAs, cookies, streaming bodies, precise timings and
cancellation. Bruno uses axios/Node HTTP with custom agents and is HTTP/1.1
only; apigo defaults to HTTP/1.1 for parity and offers HTTP/2 as an opt-in.

Alternatives:

1. `net/http` with per-request `http.Transport`/`http.Client`.
2. A third-party client (`resty`, `fasthttp`).
3. Shell out to `curl` (loses streaming/control and adds a dependency).

## Decision

Use `net/http` and `httptrace`, with a transport built per request from the
resolved settings. Disable automatic HTTP/2 negotiation by default (set
`ForceAttemptHTTP2` false and an empty `TLSNextProto`) so behavior matches
Bruno; enable HTTP/2 only when the request opts in. Use `net/http/cookiejar`
with a persistent backing store.

## Consequences

Positive:

- Stdlib, stable, no cgo, full control over TLS, proxy, redirects and timing.
- HTTP/2 available as an explicit opt-in, not a default that changes parity.
- `httptrace` provides DNS/connect/TLS/TTFB timings.
- Streaming request/response bodies and context cancellation are native.

Negative:

- Header ordering and some wire-level quirks are not controllable (and should
  not matter to servers).
- Proxies, NTLM and digest need custom `RoundTripper`/auth code.
- `net/http` normalizes some URLs; the builder must preserve semantics and be
  tested at the wire level.

Mitigations:

- A dedicated `internal/httpx` layer owns URL/body/auth assembly; tests assert
  the final `*http.Request` and, where needed, a raw test server.
- Proxy and auth are separate packages with their own integration tests.
- Large responses stream to disk; the UI never depends on a fully buffered body.

## References

- [Request engine specification](../REQUEST_ENGINE.md)
