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
- One script, `bin/install-files`, describes the installed layout; the
  packages, `make install-user` and the `PKGBUILD` all go through it, and
  `bin/check-packages` fails when a package does not hold exactly the
  staged files.
- nfpm v2.48.0 keeps the pre-release in Debian and RPM versions but drops it
  from the Arch `pkgver`, so `bin/build-packages` folds it into the Arch
  version itself (`0.12.0unstable`, `1.0.0rc2`), which pacman sorts before
  the release.
- Reproducible packages: file times are pinned to the last commit's time
  (`SOURCE_DATE_EPOCH`), so the same commit gives byte-identical packages.
- The package `license` field stays empty (`LicenseRef-unknown` in the
  `PKGBUILD`) until the maintainer chooses a license (milestone 10).

## Alternatives

- `dpkg-deb` and `rpmbuild` scripts: three toolchains to maintain. Rejected.
- Flatpak/Snap: see context. Rejected.

## Update (2026-11-07): the license

The maintainer chose the GNU General Public License, version 3 or (at
the user's option) any later version: `LICENSE` holds the official text,
the nfpm manifest and the `PKGBUILD` say `GPL-3.0-or-later`, the widget's
metadata carries the same SPDX id, and every package installs `LICENSE`
with the docs (`/usr/share/doc/ataecina/LICENSE`).
