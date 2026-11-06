# ADR 0011 — Source fidelity and isolated adapters

- Status: accepted
- Date: 2026-11-06

## Context

About half of the useful sources are undocumented and may change at any
moment. Figures also differ in trust: a provider's own percentage is not the
same as a balance computed from local records or a reset inferred from a 429.

## Decision

- Every reading carries a **fidelity** (`REPORTED`, `DERIVED`, `OBSERVED`,
  `DECLARED`), a **freshness** and an **outcome**; displays always show them.
- Each source is one adapter package with its own fixtures, fuzz target and
  error classification. An adapter failure yields `BROKEN` or `UNAVAILABLE`
  for its subscription only; the collector keeps the last good reading for
  transient errors and never fails the whole snapshot.
- Sources are added to SPECS §3 with their verification date before code is
  written.

## Consequences

- The owner always knows how much to trust each figure.
- A provider change is a one-adapter fix, typically a `fix` (PATCH).

## Alternatives

- Hiding provenance and showing a single number: simpler display, misleading
  when a figure is stale or estimated. Rejected.
