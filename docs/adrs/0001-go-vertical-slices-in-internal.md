# ADR 0001 — Go vertical slices under `internal/`

- Status: accepted
- Date: 2026-11-06

## Context

The Simplicer rules prefer a `src/<slice>/{domain,application,infrastructure}`
layout and a `tests/` tree. Go has stronger conventions: `internal/` for code
that must not be imported from outside the module, `cmd/<binary>/` for entry
points, and tests colocated with the code as `*_test.go`.

## Decision

Follow the Go standard. Production code lives in
`internal/<context>/{domain,application,infrastructure}`, the composition root
in `cmd/ataecina/`, tests next to the code. Integration, acceptance and e2e
tests use build tags (`integration`, `acceptance`, `e2e`); whole-binary e2e
tests live in `tests/e2e/`. Shared technical code lives in `internal/platform/`.

Non-Go deliverables mirror their install location: the Plasma widget under
`share/plasma/plasmoids/`, desktop files under `share/applications/`, icons
under `share/icons/`; systemd units and packaging under `infra/`.

## Consequences

- The language standard wins over the generic layout, as the precedence rules
  require; this ADR documents the exception.
- Nothing under `internal/` is a public Go API.

## Alternatives

- `src/` at the root: fights `go` tooling and import paths. Rejected.
