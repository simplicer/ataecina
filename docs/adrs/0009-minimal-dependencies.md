# ADR 0009 — Minimal dependencies

- Status: accepted
- Date: 2026-11-06

## Context

A tool that holds keys is a supply-chain target. The standard library covers
HTTP, TLS, JSON, AES-GCM, PBKDF2 (`crypto/pbkdf2`, Go 1.24+), and terminal
no-echo input through `syscall`. No SQLite driver is needed: OpenCode's
recorded costs come from its own CLI (`opencode stats --models --json`). D-Bus (Secret Service,
notifications, StatusNotifierItem, systemd) has no standard-library client.

## Decision

- Runtime dependencies: the Go standard library and
  `github.com/godbus/dbus/v5` (pure Go, widely used), pinned.
- Build-time tools (nfpm, govulncheck) are pinned and never linked.
- Any new dependency requires its own ADR. `govulncheck` runs in
  `make check`.

## Consequences

- The binary stays small and auditable.

## Alternatives

- `golang.org/x/crypto` for Argon2id and `golang.org/x/term`: reasonable, but
  unnecessary today. Rejected for now.
- `fyne.io/systray`: pulls more than needed for one StatusNotifierItem.
  Rejected.
