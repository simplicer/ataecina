# ADR 0012 — One redacting secret type shared by the contexts

- Status: accepted
- Date: 2026-11-06

## Context

AGENTS.md requires that a key redacts itself in `String()`, `GoString()`,
`Format` and `MarshalJSON`, and never appears in logs, errors or the
snapshot. Two contexts handle keys: `keys` seals and unseals them, and
`allowances` sends them to a provider through its `KeyRing` port. Two
separate types would need two sets of redaction rules and a conversion at
the boundary, where a raw `[]byte` would travel between contexts.

## Decision

- `internal/platform/secret.Key` is the single in-memory type for a
  provider key or a master key. It is a pure value: no I/O, no clock, no
  global state. It redacts itself in every textual and JSON encoding and
  exposes the bytes only through `Reveal()`, for the one place that must
  send or seal them, and `Wipe()` to overwrite them.
- Domain packages of `keys` and `allowances` may import it, as they import
  `time.Time`; it is the only `platform` package a domain may import.

## Consequences

- Redaction is implemented and tested once.
- The dependency rule keeps its meaning: `platform/secret` depends on
  nothing but the standard library and holds no business rule.

## Alternatives

- One type per context: duplicated redaction, raw bytes at the boundary.
  Rejected.
- `[]byte` or `string` everywhere: prints itself in logs and errors.
  Rejected.
