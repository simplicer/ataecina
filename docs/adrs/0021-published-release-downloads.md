# ADR 0021 — Published release downloads

- Status: accepted
- Date: 2026-11-08

## Context

Installing Ataecina meant cloning the repository, installing Go, `make`
and gettext, and building a package (ADR 0008). That is a minute for a
developer and a wall for anyone else. The packages are reproducible and
already verified by `bin/check-packages`; they only needed to be
published. Versions are still 0.x (`-unstable`, simplicer-semver), which
must never pass for a stable release.

## Decision

- `make package` also writes a release archive per architecture,
  `ataecina-X.Y.Z[-suffix]-linux-<arch>.tar.gz`: the command, the built
  widget, the unit template, `bin/install-files` and `./install`, which
  installs everything for the user into `~/.local` without root (and
  removes it with `--uninstall`). It is reproducible like the packages
  (sorted, owners 0, times of the commit, `gzip -n`) and listed in
  `SHA256SUMS`. `bin/check-packages` installs it into a throw-away `HOME`,
  runs it and removes it.
- `bin/publish-release`, run by the maintainer, publishes the version in
  `VERSION`: from a clean `main` that origin has, with `bin/check-version`
  passing and a new tag, it builds every package, tags the commit
  `vX.Y.Z[-suffix]` (annotated — the Go release, per AGENTS.md), pushes the
  tag to Codeberg and GitHub, and creates the release with every file of
  `dist/` and the version's CHANGELOG section on both.
- A version with a suffix is published as a **pre-release**; only a version
  without suffix (from `1.0.0`, through the release ceremony) is a stable
  release.
- The Codeberg token is read from a file into a `0600` header file for
  curl; it never appears on a command line, in the output or in the
  repository. GitHub goes through `gh`.

## Consequences

- An owner downloads one file and installs it with the system's package
  tool, or unpacks the archive anywhere.
- Each published version keeps its files on both forges; the tag is the
  record of what was published.
- The maintainer's machine needs `curl`, `jq` and `gh` to publish; building
  still needs only Go and `make`.
- Packages are not signed yet; `SHA256SUMS` checks integrity, the
  reproducible build lets anyone check provenance.

## Alternatives

- **An apt/dnf repository**: updates through the system, but needs signed
  repository metadata and hosting. Later, when there are stable releases.
- **Flatpak or Snap**: a Plasma widget and systemd user units do not fit
  their sandboxes.
- **A `curl | sh` installer**: runs unreviewed code as the user; the
  archive's `./install` is the same idea, readable before it runs.
- **CI-built releases**: would need the Codeberg token in CI; the
  reproducible local build gives the same bytes.
