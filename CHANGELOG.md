# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
following the Simplicer policy (`AGENTS.md`, "Versioning").

## [Unreleased]

## [0.2.0-unstable] - 2026-11-06

### Added

- `codex-logs` source: reads the rate limits Codex writes into its session
  logs, incrementally from the last byte offset, read-only and without
  storing any log content; windows are mapped by duration, every limit id
  with windows is a bucket, Codex credits and the plan are shown.

## [0.1.0-unstable] - 2026-11-06

### Added

- `allowances` domain: subscriptions, meters over five-hour, daily, weekly,
  monthly, billing-cycle and other windows (recognised by length, never by
  position), percentages that may exceed 100, balances in ISO 4217
  currencies, provider credits, banked resets, several buckets per
  subscription, fidelity, outcomes, error classes, freshness and the
  estimated rebirth of a window after its reset.
- `ataecina collect`: asks every configured subscription once, concurrently
  and within a 20-second deadline; a failing, slow or panicking source only
  affects its own subscription and transient failures keep the last good
  reading. Sources not implemented yet read as `UNSUPPORTED`.
- `ataecina status` table and `ataecina status --json`, the public JSON
  contract of SPECS §8.2, with RFC 9457 problems on stderr.
- Strict `config.json` with per-source options, XDG paths, private file and
  directory checks, atomic snapshot in `$XDG_RUNTIME_DIR`, Loki-compatible
  JSON logs with rotation, `etc/ataecina.example.json`.
- `Makefile` (`check`, `e2e`, `fuzz`, `build`), `bin/set-version`,
  `bin/check-version`, `bin/check-coverage`.

## [0.0.0-unstable] - 2026-11-06

### Added

- Project foundation: engineering rules (`AGENTS.md`), business
  specification (`docs/SPECS.md`), roadmap (`docs/ROADMAP.md`) and
  architectural decision records 0001–0011 (`docs/adrs/`).
