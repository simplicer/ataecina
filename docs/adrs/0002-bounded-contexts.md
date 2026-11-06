# ADR 0002 — Bounded contexts

- Status: accepted
- Date: 2026-11-06

## Context

Ataecina reads allowances, protects keys and warns the owner. These change
for different reasons: a provider changes its API; the way keys are held
changes; notification rules change.

## Decision

Three contexts, plus one planned:

- `allowances` — subscriptions, meters, balances, readings, fidelity,
  freshness, rebirth and the snapshot; collecting and reporting them. One
  infrastructure adapter per source.
- `keys` — sealed provider keys, the master key and its unlockers.
- `alerts` — thresholds, alert state per window, notifications.
- `outages` (planned) — provider status pages.

`allowances` asks for a key through a `KeyRing` port implemented by `keys`;
`alerts` consumes meters through an application port of `allowances`. No
context imports another context's infrastructure.

## Consequences

- A provider change touches one adapter in `allowances` only.
- `keys` can be audited on its own: it is the only code that sees a key in
  clear.

## Alternatives

- A single `quotas` package: mixes the most sensitive code (keys) with the
  most volatile (provider adapters). Rejected.
