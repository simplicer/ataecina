# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
following the Simplicer policy (`AGENTS.md`, "Versioning").

## [Unreleased]

## [0.19.0-unstable] - 2026-11-07

### Added

- A desktop notification tells once when a subscription's source starts
  failing ("workers-ai · key missing", with the source's hint), and again
  only if it fails for another reason; a source that recovers is forgotten.
  A transient failure that keeps the last good reading is not announced.

## [0.18.0-unstable] - 2026-11-07

### Added

- The widget's popup folds every subscription whose source failed, or that
  asks for something (declaring the OpenCode Zen balance), into a warnings
  block below the card instead of mixing them with what is left.

### Changed

- Every row shows what is left of each window (5 h, week, month or cycle)
  with its rebirth, not only the closest one, and the money left of its
  total (`$6.80 of $70.00`).
- Rings show only what is known, and turn grey and half transparent when the
  reading is stale; a stale Claude or Codex row says how it gets fresh
  again ("Updates when you use Claude Code").
- A subscription that answers with nothing worth showing — a free credit
  used up, or only what was spent — leaves the popup, so free providers
  without a plan show only when they report free-model limits.

## [0.17.1-unstable] - 2026-11-07

### Fixed

- OpenCode Zen is read under the collector timer too: the unit's `PATH`
  now includes `~/.local/share/pnpm/bin`, where recent pnpm versions put
  `opencode`; before, every timed run said "opencode is not on PATH".

## [0.17.0-unstable] - 2026-11-07

### Added

- The Plasma widget can live in the system tray: Plasma lists it among the
  tray's entries and opens its popup from the tray icon, so on Plasma it
  replaces `ataecina tray`, whose menu the desktop draws as plain text.

### Changed

- The widget's popup is redrawn in Ataecina's own colours:
  one card with a row per subscription — growth rings for each window, the
  name and plan, when the closest window is reborn and what is left in large
  figures — that unfolds into every window, the balance and the hint on a
  click, and a footer with when the snapshot was taken, "Refresh now" and
  the settings.
- The widget shows what is left (100 − used) instead of what is used, in
  the popup, the panel ring and the tooltip. The JSON contract is unchanged.

### Fixed

- The widget never prints a balance as "-0.00", and a billing cycle with no
  length is drawn as the outer, longest ring.

## [0.16.2-unstable] - 2026-11-07

### Fixed

- The collector timer runs on Ubuntu: its unit set five options that drop
  capabilities (`CapabilityBoundingSet=`, `PrivateDevices=`,
  `ProtectKernelModules=`, `ProtectKernelLogs=`, `ProtectClock=`), which a
  user manager refuses when unprivileged user namespaces are forbidden, so
  every run stopped with `218/CAPABILITIES`. They are removed; the collector
  holds no capability and the system call filter already covers them.

### Security

- ADR 0005 and `SECURITY.md` now say which hardening applies where: on
  Ubuntu the read-only home and private `/tmp` are skipped by systemd, while
  `NoNewPrivileges` and the seccomp filters apply on every distribution.
- `make e2e` starts the built binary under the user manager with the unit's
  hardening, so a refused option fails the suite instead of the timer.

## [0.16.1-unstable] - 2026-11-07

### Fixed

- The tray launcher is offered on Plasma too: it was hidden there, so the
  tray could only be started from a terminal on the reference desktop.

### Added

- `make` alone compiles the binary and stages the widget; `make deb` builds
  only the `.deb` of this machine; `make install-deb` installs it with apt
  and `make uninstall-deb` removes it.
- Every `.deb` of the host's architecture is now installed, run and purged
  in a throw-away root (`fakeroot dpkg`) and its dependencies are checked
  with `apt-get --simulate` before it is accepted.

### Added

- `SECURITY.md` (how to report a vulnerability and what Ataecina promises)
  and `CONTRIBUTING.md` (how a change is made, tested and committed).
- Forgejo Actions workflow running `make check`, `make e2e` and
  `make package` on every push to `main` and every pull request.

## [0.16.0-unstable] - 2026-11-06

### Added

- Heimdall (`heimdall`): the windows Heimdall observes for the plans of one
  provider, from `GET /v1/plans` on its admin API on `127.0.0.1` (port
  configurable, default 4001) with a sealed admin token; one bucket per
  plan, an exhausted plan at 100 %, unavailable reasons in the hint, and
  `UNAVAILABLE` with the next resets while Heimdall reports no usage.
- ADR 0013: Heimdall is read on loopback, the only plain-HTTP traffic.

## [0.15.0-unstable] - 2026-11-06

### Added

- GitHub Copilot (`copilot-gh`): this month's Copilot premium requests
  from GitHub's billing usage report, read through the official `gh api`
  with gh's own login, against the monthly allowance declared in
  `config.json` (`login`, `monthly_allowance`); the overage spent as the
  balance.
- Alibaba Model Studio Token Plan (`alibaba-cli`): every window
  `bl usage token-plan --output json` reports (5 hours, week, month).
- One runner for official CLIs (fixed argv, no shell, timeout, capped
  output), now shared by OpenCode, gh and bl; `doctor` warns when gh or bl
  is not on PATH.

## [0.14.0-unstable] - 2026-11-06

### Added

- Ollama Cloud (`ollama-api`): session and weekly usage from the
  ollama.com usage API, without reset instants.
- Orca Router (`orcarouter-api`): what is left of the key's allowance,
  what was spent and when the allowance expires, from its two billing
  summaries.
- Cloudflare Workers AI (`cloudflare-api`): today's neurons against the
  10 000 granted per UTC day, from one fixed GraphQL Analytics query with
  an analytics-only token and the account id (`"account"`).
- Configuration options `account`, `login`, `monthly_allowance` and `port`,
  each accepted only by its source and required where the source needs it.

## [0.13.0-unstable] - 2026-11-06

### Added

- `ataecina tray`: a StatusNotifierItem with a DBusMenu for XFCE, Cinnamon,
  MATE, LXQt, Budgie, Waybar and GNOME with the AppIndicator extension. One
  menu line per subscription, the snapshot's age, "Refresh now" (also a
  middle click) and "Quit"; attention from 90 %; a tooltip with every line.
  It registers when the panel's watcher appears and ends on Quit, SIGINT or
  SIGTERM.
- "Refresh now" asks the user's systemd manager over the session bus to
  start `ataecina-collect.service` (no program is run).
- Launcher `org.simplicer.ataecina.desktop` (hidden on Plasma), installed
  by the packages and `make install-user` with `Exec` naming the installed
  binary.

### Added

- `make package`: `.deb`, `.rpm` and Arch packages for amd64 and arm64 built
  with nfpm (pinned, never linked) from `infra/packaging/nfpm.yaml`,
  byte-reproducible from the same commit, listed in `dist/SHA256SUMS` and
  verified by `bin/check-packages` (contents, modes, unit path, versions).
- `make install-user` and `make uninstall-user` (into `~/.local`, units in
  `~/.local/share/systemd/user`); `bin/install-files` is the single
  description of the installed layout, shared with the packages and the
  AUR `PKGBUILD` (`infra/packaging/arch/`).
- `bin/set-version` and `bin/check-version` keep the `PKGBUILD` tag and
  `pkgver` in step with `VERSION`.

## [0.12.0-unstable] - 2026-11-06

### Added

- Plasma 6 widget `org.simplicer.ataecina`: a ring around the icon in the
  panel showing the worst meter (accent, warning from 90 %, critical from
  95 %, grey when nothing is fresh), a popup with a bar and a countdown to
  rebirth per window, balances, credits, fidelity and outcome badges, and
  "refresh now" through `ataecina-collect.service`; a settings page for the
  command, the refresh interval, the panel percentage and which
  subscriptions to show in which order.
- Spanish translation of the widget; `make widget` stages it with compiled
  translations, `make check` lints it with `qmllint` when installed and
  checks the translations with `msgfmt`.
- `tests/widget`: static checks that keep the widget's metadata at
  `VERSION`, its settings schema in step with the QML, every string
  translated and every field it reads inside the `status --json` contract.

## [0.11.0-unstable] - 2026-11-06

### Added

- Hardened systemd user units `ataecina-collect.service` and
  `ataecina-collect.timer` (every five minutes), verified with
  `systemd-analyze`.
- `systemd-creds` unlocker: the master key as a TPM-bound user credential,
  loaded into the collector unit through a drop-in; `doctor` checks that the
  timer is installed.

## [0.10.0-unstable] - 2026-11-06

### Added

- `alerts` context: after each collection, every threshold a meter crosses
  (default 90 and 95 %, configurable per subscription) is announced once
  per window through `org.freedesktop.Notifications`, the highest only when
  several are crossed at once; the state lives in `alerts.json` and is
  forgotten when the window is reborn. Without a session bus alerts are
  logged.

## [0.9.0-unstable] - 2026-11-06

### Added

- API-key sources `openrouter-api`, `vercel-api`, `zai-api`, `minimax-api`
  and `commandcode-api`, reading with the sealed key of each subscription;
  `collect --passphrase-fd`.
- Hardened HTTP client (ADR 0006): one compiled-in HTTPS origin per
  adapter, no redirects, no proxy unless enabled, 5 s connect and 10 s
  request timeouts, 64 KiB bodies after decompression, `ataecina/<version>`
  User-Agent, no retries; `Retry-After` is honoured.

## [0.8.0-unstable] - 2026-11-06

### Added

- `ataecina doctor`: checks permissions, the configuration, `keys.json` and
  its unlockers, which process provides the Secret Service, the key of every
  subscription that needs one, the official CLIs on `PATH` and the snapshot;
  read-only, exits 1 when something is broken.

## [0.7.0-unstable] - 2026-11-06

### Added

- `secret-service` unlocker: the master key is an item of the session's
  Secret Service (GNOME Keyring, KWallet/ksecretd, KeePassXC) over D-Bus;
  `ataecina init` uses it by default when it is reachable. Commands a person
  runs may show the keyring's unlock prompt; the collector never does.
- Dependency: `github.com/godbus/dbus/v5` v5.2.4 (ADR 0009).

## [0.6.0-unstable] - 2026-11-06

### Added

- `keys` context: `ataecina init`, `ataecina keys set|list|forget` and
  `ataecina unlockers list|add|remove`. Provider keys are sealed with
  AES-256-GCM under a random master key, bound to their name, in a private
  `keys.json`; the master key is held by a passphrase (PBKDF2-HMAC-SHA256,
  600 000 iterations) and opened at most once per process. Keys are read
  from the terminal without echo or from a pipe, never from argv; scripts
  can pass passphrases with `--passphrase-fd`.
- ADR 0012: one redacting secret type shared by the contexts.

## [0.5.1-unstable] - 2026-11-06

### Fixed

- `codex-logs` no longer reads as `UNAVAILABLE` when Codex has not been used
  for more than 8 days: older logs are read once, newest first, until the
  `codex` bucket shows its windows. A snapshot logged with null windows or
  credits no longer erases the figures already known.

## [0.5.0-unstable] - 2026-11-06

### Added

- `ataecina balance declare <subscription> (--top-up | --balance) <amount>`
  and the `opencode-zen` source: the OpenCode Zen balance is derived from
  the declared top-ups minus the cost OpenCode records for its `opencode`
  provider, read from `opencode stats` lifetime statistics with a fixed
  argv; every declaration keeps the spending to date as its baseline.
  Without a declaration the spending is shown as `PARTIAL`.

### Changed

- Official CLIs (`codex`, `opencode`) are looked up in the absolute
  directories of `PATH` by one shared helper.

## [0.4.0-unstable] - 2026-11-06

### Added

- `ataecina hook claude-statusline [--print] [--subscription <id>]` and the
  `claude-statusline` source: the hook keeps only the `rate_limits` Claude
  Code passes to its status line, merges them so an idle session replaying
  old figures never rewinds them, writes a private signal file in
  `$XDG_RUNTIME_DIR` only when something is newer, always exits 0 and runs
  in about 2 ms; `--print` writes `5h 23% · 7d 41% · ↻ 2h10m`.

## [0.3.0-unstable] - 2026-11-06

### Added

- `codex-app-server` source and the `app_server` option of `codex-logs`:
  when the logs fail or are older than `app_server_after_minutes`, the
  official `codex app-server` is asked over stdio JSON-RPC with a method
  allowlist enforced in code (`initialize`, its `initialized`
  acknowledgement, `account/rateLimits/read`); banked resets are shown as
  information only. The logs remain the source whenever it fails.

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
