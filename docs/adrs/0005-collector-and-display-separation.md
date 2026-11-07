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
- `PATH` is set to `~/.local/bin`, `~/.local/share/pnpm`,
  `~/.local/share/pnpm/bin` (where recent pnpm versions put global CLIs;
  added 2026-11-07 after `opencode` was not found under the timer) and the
  system directories so the collector finds `codex` and `opencode`.
- The units pass `systemd-analyze --user verify` in `make e2e`. Options
  that need unprivileged user namespaces must still be checked by hand on
  distributions that restrict them.

## Update (2026-11-07): what applies where unprivileged user namespaces are forbidden

Checked by hand on Ubuntu with KDE Plasma 6.6 and systemd 259, where
AppArmor forbids unprivileged user namespaces
(`kernel.apparmor_restrict_unprivileged_userns=1`, Ubuntu's default):

- A user manager drops capabilities only inside a user namespace. Without
  one, `CapabilityBoundingSet=`, `PrivateDevices=`, `ProtectKernelModules=`,
  `ProtectKernelLogs=` and `ProtectClock=` each stopped the collector with
  `218/CAPABILITIES`, so the timer never ran. They are removed. Nothing is
  lost: the collector holds no capability, `NoNewPrivileges` keeps it so,
  and `@system-service` already denies the clock, module, raw I/O and
  kernel log system calls they also filtered.
- `ProtectSystem=strict`, `ProtectHome=read-only`, `PrivateTmp`,
  `ProtectKernelTunables`, `ProtectControlGroups` and `ProtectHostname`
  need a mount or UTS namespace. On Ubuntu systemd skips them (only
  `ProtectHostname` says so in the journal): the collector can write to the
  home and sees the real `/tmp`. They stay in the unit because they apply on
  distributions that allow unprivileged user namespaces (the default on
  Debian, Fedora and Arch; not checked by hand there).
- `NoNewPrivileges`, `RestrictAddressFamilies`, `RestrictNamespaces`,
  `RestrictRealtime`, `RestrictSUIDSGID`, `LockPersonality`,
  `SystemCallArchitectures`, `SystemCallFilter` and `UMask` use no
  namespace and apply everywhere.
- `make e2e` now starts the built binary as a transient user unit with the
  service's hardening (`TestTheCollectorStartsUnderTheUserManager`), so a
  refused option fails the suite on any machine with a user manager instead
  of the owner's timer.
- The read-only home on Ubuntu is left to the Landlock self-sandbox already
  listed in `docs/ROADMAP.md` ("Later"), which needs no user namespace.

