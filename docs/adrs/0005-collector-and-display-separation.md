# ADR 0005 — Collector and display are separate

- Status: accepted
- Date: 2026-11-06

## Context

The parts that draw figures (Plasma widget, tray, terminal) run often and
inside the desktop shell. The part that reads providers needs the network and
the keys. Merging them gives the most exposed code the most privileges.

## Decision

- **Collector**: `ataecina collect`, one-shot, run by
  `ataecina-collect.timer` (default every 5 minutes) as a hardened systemd
  user service (`NoNewPrivileges`, `ProtectSystem=strict`,
  `ProtectHome=read-only` with write access only to Ataecina's runtime and
  state directories, `PrivateTmp`, `RestrictAddressFamilies=AF_UNIX AF_INET
  AF_INET6`, `SystemCallFilter=@system-service`, `MemoryDenyWriteExecute`,
  `UMask=0077`). It is the only code that unseals keys or opens sockets to
  providers.
- **Snapshot**: `$XDG_RUNTIME_DIR/ataecina/snapshot.json`, figures only,
  written atomically, `0600`, on tmpfs (gone at logout).
- **Displays** read the snapshot only. "Refresh now" starts the collector
  unit through systemd's D-Bus API; displays never collect themselves.

## Consequences

- A bug in the widget or tray cannot leak a key: they never hold one.
- Nothing stays resident except the optional tray.
- Some hardening options need unprivileged user namespaces; on distributions
  that restrict them (Ubuntu's AppArmor policy) the unit must be verified and
  the options that cannot apply documented.

## Alternatives

- A resident daemon serving both: simpler wiring, larger attack surface,
  constant memory. Rejected.

## Update (2026-11-06): the units as shipped

- `infra/systemd/ataecina-collect.service.in` (`@BINDIR@` replaced at
  install) and `ataecina-collect.timer` (every 5 minutes, first run one
  minute after the user manager starts).
- `StateDirectory=ataecina` and `RuntimeDirectory=ataecina` with
  `RuntimeDirectoryPreserve=yes` (all `0700`) give the collector its own
  writable state and keep the snapshot between runs; the rest of the home
  is read-only, except what the official CLIs write while answering
  (`~/.codex`, OpenCode's data, state and cache), each optional.
- `MemoryDenyWriteExecute=` is **not** set: `opencode` runs on a JIT and
  would crash. `ProtectHome=read-only`, `ProtectSystem=strict`,
  `PrivateTmp`, `PrivateDevices`, the kernel and clock protections,
  `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6`,
  `SystemCallFilter=@system-service`, `NoNewPrivileges`, an empty
  capability set and `UMask=0077` are.
- `PATH` is set to `~/.local/bin`, `~/.local/share/pnpm` and the system
  directories so the collector finds `codex` and `opencode`.
- The units pass `systemd-analyze --user verify` in `make e2e`. Options
  that need unprivileged user namespaces must still be checked by hand on
  distributions that restrict them.

