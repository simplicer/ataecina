# ADR 0006 — Outbound network policy

- Status: accepted
- Date: 2026-11-06

## Context

Each request carries a key that can spend money. A misconfiguration, a
redirect, a proxy variable or a compromised endpoint must not be able to send
a key anywhere else.

## Decision

- Each source adapter has a fixed HTTPS scheme and host compiled in. The
  production binary has no way to override them; fake endpoints for tests
  exist only in binaries built with the `e2e` tag.
- Redirects are refused (`CheckRedirect` returns an error).
- `HTTP_PROXY`/`HTTPS_PROXY` are ignored unless the owner sets
  `"network": { "use_environment_proxy": true }` in `config.json`.
- Timeouts: connect 5 s, whole request 10 s; a global collector deadline.
- Response bodies are capped at 64 KiB after decompression; larger answers
  are `CONTRACT_CHANGED`.
- One attempt per source per collection; no retries. `429`/`Retry-After`
  pauses that source until the announced time.
- `User-Agent: ataecina/<version>`; no other client's identity.
- No telemetry, analytics or update checks.

## Consequences

- A key can only ever reach its own provider's host.
- Corporate proxies need an explicit opt-in.
- The one exception to "fixed HTTPS host" is the owner's Heimdall on
  `127.0.0.1` over plain HTTP, port configurable (ADR 0013).

## Alternatives

- Configurable base URLs for "flexibility": would let a typo or an attacker
  redirect keys. Rejected.
