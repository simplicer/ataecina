# ADR 0010 — JSON output contract and no HTTP surface

- Status: accepted
- Date: 2026-11-06

## Context

Widgets and scripts need machine-readable output. A local HTTP API would be a
listening socket on a machine that holds keys, and every extra listener is
attack surface.

## Decision

- No HTTP server. Displays use `ataecina status --json` or read the snapshot.
- The JSON output is a public, versioned contract (SPECS §8.2) following the
  Zalando JSON guidelines: `snake_case` properties, `UPPER_SNAKE_CASE` enums,
  RFC 3339 UTC date-times, money objects with ISO 4217 currency, a
  `data`/`meta` envelope, and RFC 9457 problem objects for errors. Changes
  are additive; removals and renames are breaking.
- If an HTTP surface is ever needed it requires a new ADR, must be
  loopback-only and authenticated, contract-first in OpenAPI, and follow the
  Zalando RESTful guidelines with the Simplicer exceptions (`PUT` with client
  UUID for idempotent creation, rare `:action` commands).

## Consequences

- The widget and the binary are coupled only by a documented document.
- Nothing listens on the network.

## Alternatives

- A loopback REST API for widgets: convenient, but a socket for nothing the
  JSON command cannot do. Rejected.
