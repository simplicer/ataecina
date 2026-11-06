# ADR 0008 — Native packages built with nfpm

- Status: accepted
- Date: 2026-11-06

## Context

Ataecina must install cleanly with the native package manager of the main
families: Debian/Ubuntu (`.deb`), Fedora/openSUSE (`.rpm`) and Arch. The
binary is static, so packages only place files: the binary, systemd user
units, the plasmoid, the desktop file, icons and docs. Flatpak and Snap
sandboxes conflict with reading `~/.codex` and Claude's status line, with the
tray and with the Secret Service, and would add no protection here.

## Decision

- `make package` builds `.deb`, `.rpm` and the Arch package from one manifest
  (`infra/packaging/nfpm.yaml`) with nfpm, pinned and run as a build-time tool
  (never linked into the binary).
- An AUR `PKGBUILD` (`infra/packaging/arch/PKGBUILD`) builds from the release
  tag for Arch users who prefer source builds.
- Version mapping: `X.Y.Z-unstable` → `X.Y.Z~unstable` for Debian and RPM so
  pre-releases sort before the release; Arch uses its own allowed syntax.
- Releases publish checksums signed by the maintainer.

## Consequences

- No distribution-specific build machines are needed.
- Packages declare no runtime dependency beyond systemd.

## Alternatives

- `dpkg-deb` and `rpmbuild` scripts: three toolchains to maintain. Rejected.
- Flatpak/Snap: see context. Rejected.
