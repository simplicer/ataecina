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
