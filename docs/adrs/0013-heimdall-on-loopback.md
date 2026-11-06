# ADR 0013 — Heimdall is read on loopback

- Status: accepted
- Date: 2026-11-06

## Context

Several free tiers (Gemini API keys, NVIDIA, Groq, LLM7, Kilo Gateway) have
no endpoint that reports their allowance; the owner's model relay, Heimdall,
observes their cooldowns and resets while it serves them and publishes them
on its admin API (`GET /v1/plans`, OpenAPI `docs/openapi/admin.yaml`). That
API listens on `127.0.0.1:4001` by default, over plain HTTP, and the owner
may move it to another port. ADR 0006 allows only fixed HTTPS hosts compiled
into each adapter.

## Decision

- The `heimdall` source talks to `http://127.0.0.1:<port>` only. The host is
  fixed in the binary (`httpclient.Client.ForLoopback`); the owner chooses
  only the port (`"port"`, default 4001). No other host, name or scheme can
  be configured.
- Everything else of ADR 0006 still applies: no redirects, capped bodies,
  timeouts, one attempt, `Retry-After`, the plain `ataecina/<version>`
  User-Agent. Go never proxies loopback traffic.
- The admin token is a sealed key like any provider key (`ataecina keys set
  heimdall`), sent as `Authorization: Bearer`.
- Ataecina follows Heimdall's contract: `window_used_ratio`,
  `next_reset_at`, `available` and `unavailable_reason`. While a Heimdall
  version does not report the usage, the subscription reads `UNAVAILABLE`
  with the next reset in its hint; nothing is estimated.

## Consequences

- The only plain-HTTP traffic Ataecina sends never leaves the machine.
- A process that takes over the port could receive the admin token; the
  same is true of Heimdall's own clients, and the token only manages
  Heimdall.
- The collector unit's `RestrictAddressFamilies` already allows `AF_INET`.

## Alternatives

- Read Heimdall's database or logs directly: couples Ataecina to Heimdall's
  internals. Rejected.
- A configurable URL: would let a configuration send the token anywhere.
  Rejected.
- Require HTTPS on loopback: Heimdall's admin listener has no certificate,
  and TLS adds nothing on 127.0.0.1. Rejected.
