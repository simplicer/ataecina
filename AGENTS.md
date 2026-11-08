# AGENTS.md — Ataecina

Rules for every person and every agent working on this repository. They are
mandatory. The business specification is `docs/SPECS.md`, the plan is
`docs/ROADMAP.md` and every architectural decision lives in `docs/adrs/`
(this file states the rules; the ADRs keep the reasons, never duplicated here).

**Precedence.** This file overrides any `AGENTS.md` or `CLAUDE.md` found in
parent directories (for example a generic PHP/Slim template in `~/CODE/`)
wherever they differ: this is a Go project with no PHP, Slim, PostgreSQL,
Redis, RabbitMQ, Behat or Playwright. The owner's commit rules below
(no AI attribution) apply on top of any tool default.

## What this system is

`ataecina` is a single Go binary for Linux that tells its owner how much is
left of each AI subscription and balance they use, and when each quota window
is reborn (resets). It:

1. collects **readings** of every configured **subscription** from the safest
   source available: local signals written by the official tools, official
   CLIs, or provider APIs with the owner's own API keys;
2. keeps provider keys **sealed** at rest in `~/.config/ataecina/keys.json`,
   unlocked through the desktop Secret Service, a passphrase or systemd
   credentials;
3. shows the **snapshot** in the terminal, as JSON, in a KDE Plasma widget
   (first-class) and in a StatusNotifierItem tray for other desktops;
4. raises **alerts** when a meter crosses its thresholds;
5. keeps a daily **model catalogue** per subscription (which models, which
   free, which new) from the public models.dev catalogue (ADR 0015);
6. ships as `.deb`, `.rpm` and an Arch package, plus a source build.

The domain is **subscription allowances and their renewal** — not HTTP, not
D-Bus, not desktops.

## Domain vocabulary

Use these words verbatim in code, docs and commits (defined in
`docs/SPECS.md` §1): subscription, provider, allowance, meter, window,
balance, reading, source, fidelity, freshness, snapshot, reset (rebirth),
threshold, alert, sealed key, unlocker, collector, display, offering,
model catalogue, peak hours, off-peak.

## Architecture

- Domain-Driven Design, vertical slicing, Screaming Architecture, Clean
  Architecture dependency direction.
- **Go standard overrides the generic `src/` layout** (ADR 0001): production
  code under `internal/`, entry point under `cmd/ataecina/`, tests colocated
  as `*_test.go`.
- Bounded contexts, each `internal/<context>/{domain,application,infrastructure}`
  (ADR 0002):

| Context | Owns | Must not |
|---|---|---|
| `allowances` | subscriptions, meters, balances, readings, fidelity, freshness, the snapshot; collecting and reporting them | hold or unseal keys; draw anything |
| `keys` | sealed provider keys, unlockers, sealing and unsealing | know what a quota is |
| `alerts` | thresholds, alert state per window, notifications | collect readings |
| `offerings` | the model catalogue: which models each provider offers, their list price, which are free and which are new; refreshing and reporting it | read quotas, hold keys, send any credential |
| `outages` *(planned)* | provider status pages and incidents | read quotas |

- `internal/platform/` holds shared technical infrastructure only (hardened
  HTTP client, D-Bus connection, XDG paths, file permission checks, logging,
  clock). No business rules there.
- `cmd/ataecina/` is the composition root: CLI subcommands and wiring only.
- Non-Go deliverables mirror their install location (LFS/FHS style):
  `share/plasma/plasmoids/org.simplicer.ataecina/` (Plasma widget),
  `share/applications/`, `share/icons/`, `infra/systemd/` (user units),
  `infra/packaging/` (nfpm manifest, PKGBUILD).

### Layer rules

**Domain** — pure Go. No I/O, no `net/http`, no D-Bus, no `os`, no
filesystem, no provider payload types, no clock reads (time is passed in).
Entities, value objects, domain services, domain errors, repository/port
interfaces.

**Application** — use cases only, one per file named after the business
action: `collect_readings.go`, `report_snapshot.go`,
`record_claude_limits.go`, `declare_balance.go`, `seal_key.go`,
`forget_key.go`, `raise_threshold_alerts.go`. Each use case exposes a single
public method. It orchestrates the domain through ports. No HTTP, no D-Bus,
no filesystem, no environment variables, no config loading.

**Infrastructure** — adapters: one source adapter per provider
(`internal/allowances/infrastructure/<source>/`), the snapshot file, the
sealed key file, the Secret Service / passphrase / systemd-creds unlockers,
desktop notifications, the tray, terminal and JSON renderers, CLI wiring.

Dependency direction is always `infrastructure → application → domain`
(and `infrastructure → domain`). Never the opposite. A context uses another
context only through that context's application ports; never its
infrastructure. `allowances` obtains a key through a `KeyRing` port that
`keys` implements; it never sees the sealed file or the unlockers.

### Adding a provider (the common task)

1. Document the source in `docs/SPECS.md` §3 first: host, path, credential,
   fields read, fidelity, whether it is documented, date verified.
2. One adapter package under `internal/allowances/infrastructure/<source>/`
   implementing the `Source` port: fixed HTTPS host, fields mapped to domain
   meters/balances, provider errors classified into domain errors.
3. A sanitized, recorded response fixture (no real key, account id, e-mail
   or amount that identifies the owner) and integration tests against an
   `httptest` server; a fuzz target for the response decoder.
4. Register it in the composition root; it is enabled only when the owner
   lists the subscription in `config.json`.
5. A broken or changed provider must surface as `BROKEN` for that
   subscription only; it never breaks the snapshot (ADR 0011).

### Forbidden names

As top-level concepts or package names: `utils`, `helpers`, `services`,
`models`, `handlers`, `controllers`, `repositories`, `managers`, `common`,
`misc`. They may appear only inside an infrastructure adapter when a
platform requires it.

## Security rules — non-negotiable

Ataecina handles credentials that can spend money. These rules win over any
feature (ADRs 0003–0006).

- **Never read, refresh, store or forward OAuth tokens, session cookies or
  browser storage** of any provider, and never run a vendor login flow or
  scrape a web console. Claude and Codex are read from the signals their own
  tools write locally; other providers only through API keys or their
  official CLIs.
- **Never imitate a client.** The `User-Agent` is `ataecina/<version>`; no
  foreign headers, no TLS fingerprint tricks.
- **Keys** enter only through `ataecina keys set <provider>` reading stdin
  without echo. The Plasma widget's Keys settings page may feed it from
  KDE's masked password dialog through an anonymous pipe, on an explicit
  click (ADR 0014); the widget's own code never reads or holds the key.
  They never appear in argv, environment dumps, logs, the
  snapshot, error messages, fixtures, test names, commits or issue reports.
  The key type redacts itself in `String()`, `GoString()`, `Format` and
  `MarshalJSON`.
- **At rest** keys live only in `~/.config/ataecina/keys.json`, each sealed
  with AES-256-GCM under a random master key, with the provider id as
  associated data. The master key is held by one or more unlockers (Secret
  Service, passphrase, systemd-creds); it is never written in clear.
- **Permissions**: `~/.config/ataecina/` is `0700`, `config.json` and
  `keys.json` are `0600` and owned by the user; Ataecina refuses to run
  otherwise, like ssh. Every file it creates uses umask `0077`.
- **Privilege separation** (ADR 0005): only the collector (`ataecina
  collect`, run by a hardened systemd user timer) opens network connections
  and unseals keys. The display paths (`status`, `--json`, tray, Plasma
  widget) only read the snapshot, and hooks (`hook claude-statusline`) only
  write a signal file; none of them unseals a key or opens a socket other
  than the session D-Bus. The widget's settings page only starts
  `ataecina keys set` or `balance declare` as separate processes when the
  owner asks (ADR 0014), and `ataecina update` when the owner clicks
  "Update models" (ADR 0016).
- **Snapshot** holds only non-secret figures (percentages, amounts, dates,
  states) in `$XDG_RUNTIME_DIR/ataecina/snapshot.json` (`0600`, tmpfs).
- **Outbound network** (ADR 0006): each adapter has a fixed HTTPS host
  compiled in (the only exception: Heimdall on `127.0.0.1` over plain HTTP,
  where the owner chooses the port, ADR 0013); production builds cannot
  override it; redirects are refused;
  proxy environment variables are ignored unless the owner enables them in
  `config.json`; every response body is capped (64 KiB after
  decompression; only the public model catalogue, fetched without any
  key, may reach 24 MiB, ADR 0015); timeouts are explicit; one attempt per refresh, no
  retries; a `429` or `Retry-After` is honoured by skipping that source
  until then.
- **Local data of other tools** (Codex logs, Claude status line input,
  `opencode stats` output) is read-only and only the documented fields
  are extracted. Official CLIs run one at a time (ADR 0018), with a fixed
  argv, no shell and a timeout; only figures extracted from them are kept.
  Prompt or response content never leaves the process and is never stored.
- **No telemetry, no update checks, no analytics.** The only hosts contacted
  are the ones listed in `docs/SPECS.md` §3, and only for enabled
  subscriptions, plus the public model catalogue `models.dev` (§11), at
  most once a day and without any key, and `artificialanalysis.ai`
  (intelligence index, owner's key) and the published offers on
  `codeberg.org` (no key) only through `ataecina update` (ADR 0016,
  ADR 0017).
- When a provider says stop (rate limit, invalid key, policy), Ataecina
  stops asking until the time it announces.

## Local HTTP surface

There is **none**, by design (ADR 0010). The widget and the tray read the
snapshot or run `ataecina status --json`. If an HTTP surface is ever added
it must be loopback-only, authenticated, contract-first in
`docs/openapi/ataecina.yaml`, and follow the Zalando RESTful API Guidelines
with the Simplicer exceptions: `/v1/{plural-resources}/{id}` in kebab-case,
`PUT /v1/{resources}/{uuid}` for idempotent creation (`201` first, `200`
equivalent replay, `409` incompatible replay), `POST
/v1/{resources}/{id}:{action}` only for rare explicit domain commands,
`application/problem+json` (RFC 9457) for every error, `page`/`page-size`
pagination with a `data`/`meta`/`links` envelope, and `/status` plus
`/version` (`{"data":{"version":"x.y.z"}}`).

## Output contracts

- `ataecina status --json` is a public contract (ADR 0010), documented in
  `docs/SPECS.md` §8 and versioned with the project. It follows the Zalando
  JSON rules: `snake_case` property names, `UPPER_SNAKE_CASE` enum values,
  RFC 3339 date-times in UTC, money as `{ "amount": <decimal>, "currency":
  "<ISO 4217>" }`, a `data`/`meta` envelope, and errors as RFC 9457 problem
  objects on stderr. Changes are additive; removing or renaming a field is a
  breaking change.
- The terminal table is for humans and may change freely.

## Configuration and paths (XDG)

Runtime paths follow the XDG Base Directory specification, which overrides
the repository's `etc/`/`var/` convention for an installed desktop tool:

| What | Where |
|---|---|
| Configuration (no secrets) | `$XDG_CONFIG_HOME/ataecina/config.json` (default `~/.config/ataecina/`) |
| Sealed keys | `$XDG_CONFIG_HOME/ataecina/keys.json` |
| Snapshot | `$XDG_RUNTIME_DIR/ataecina/snapshot.json` |
| Hook signals (Claude limits) | `$XDG_RUNTIME_DIR/ataecina/signals/` |
| Alert state, declared balances, model catalogue, kept CLI figures | `$XDG_STATE_HOME/ataecina/` |
| Logs | `$XDG_STATE_HOME/ataecina/ataecina.log` |

In the repository, `etc/ataecina.example.json` is the documented example
configuration and `var/` holds local development state (git-ignored).
Strict validation at startup: unknown keys are errors.

## Logging

- `log/slog` JSON handler, Loki-compatible, one event per line, to stderr
  and to the log file (size-capped and rotated).
- One line per collected subscription: `subscription`, `source`,
  `fidelity`, `outcome`, `error_class`, `latency_ms`, `freshness`.
- Never log keys, `Authorization` headers, response bodies or file contents.

## Performance rules

- The display path does no network and no decryption; it reads one small
  file. `ataecina status` must finish in well under 50 ms.
- The collector is one-shot: it runs, writes the snapshot and exits. No
  resident process except the optional tray.
- Local logs are read incrementally (byte offsets), never re-scanned.
- Benchmarks for the snapshot read and the log readers run once in
  `make check` (`bench-smoke`).

## Testing philosophy

Four levels, all required before a task is complete:

1. **Unit** (`go test ./...`) — every use case with fake ports; no network,
   no filesystem, no D-Bus. Coverage of `application` packages is 100%.
2. **Integration** (`-tags integration`) — adapters against `httptest`
   servers with recorded, sanitized provider fixtures; the sealed key file
   and the snapshot in a temp dir; the Secret Service unlocker against a
   private `dbus-daemon` with a fake secret service.
3. **Acceptance** (`-tags acceptance`) — whole flows across contexts: collect
   → snapshot → status; a broken provider isolated; stale readings; a window
   reborn after its reset; alerts raised once per window; keys sealed and
   unsealed through each unlocker.
4. **E2E** (`make e2e`) — the built binary end to end in a throw-away
   `HOME`/XDG tree against fake providers. Fake endpoints are reachable only
   in a binary built with the `e2e` build tag; production builds cannot
   redirect a key. The Plasma widget is linted (`qmllint`) in `make check`
   and checked by hand in `plasmoidviewer` before releases.

The suite never calls a real provider and never touches the developer's real
keys or `~/.config/ataecina`. Checks against real providers are manual, with
your own keys. Provider response decoders and the status line hook input are
fuzzed: their seed corpus runs in every `go test`, `make fuzz` explores
further. `govulncheck` runs in `make check`.

## Naming

- Directories: kebab-case where Go allows; Go package names are lowercase
  single words (`allowances`, not `allowance-readings`) — the language
  standard wins.
- Go files: `snake_case.go`; tests `*_test.go` colocated.
- Scripts in `bin/`: kebab-case (`bin/set-version`, `bin/check-version`).
  There is no `scripts/` directory.

## Errors

- Errors are values wrapped with `%w`; domain errors are declared in
  `domain/`.
- Provider failures are classified in infrastructure into the domain error
  classes of `docs/SPECS.md` §4.3 (`KEY_REJECTED`, `RATE_LIMITED`,
  `UNAVAILABLE`, `CONTRACT_CHANGED`, `UNREADABLE`); the class drives what the
  display shows and when the source is asked again.
- No panics outside startup validation.

## Versioning (simplicer-semver)

- `VERSION` is the single source of truth. Major 0 ⇒ every version is
  `0.Y.Z-unstable`. The first stable release is `1.0.0`, reached only
  through `feat!: release candidate 1.0.0` and `feat: release`, both
  explicitly requested by the maintainer.
- `feat` → MINOR +1 (PATCH 0); `fix`, `refactor`, `perf` → PATCH +1;
  `docs`, `chore`, `test`, `ci`, `build`, `style`, `revert` → no change.
  A `!` never bumps MAJOR.
- Propagate the version in the same commit to `VERSION`, `CHANGELOG.md`
  (Keep a Changelog), `README.md` and the Plasma widget `metadata.json`
  (`KPlugin.Version`). Packages read `VERSION` at build time; the package
  scripts map `-unstable`/`-rc.N` to each format's pre-release syntax
  (`~unstable` for Debian and RPM). Go has no manifest version: the
  annotated git tag `vX.Y.Z[-suffix]` is the release.
- `bin/check-version` (run by `make check`) fails on drift.
- Trunk-based: all work on `main`, linear history (rebase/squash), releases
  frozen on `release/X.Y.Z` branches. `-unstable` is never published as a
  stable package; when the maintainer asks, `bin/publish-release`
  publishes it as a marked pre-release on Codeberg and GitHub, so that
  anyone can download it (ADR 0021).

## Commits and authorship

- **One task = one Conventional Commit in English**
  (`type(scope): imperative summary`), subject ≤ 72 characters.
- The body always states: **What** changed, **Why**, **Tests** added,
  **Architectural impact**, **Version impact**.
- **The maintainer is the only author and the only responsible party.**
  Commits are authored and committed with the repository's local git
  identity. **Never add AI attribution of any kind**: no
  `Co-Authored-By` trailer for an AI agent or model, no "Generated with…"
  line, no agent or model name in subjects, bodies, trailers, branch names,
  tags, CHANGELOG entries, release notes or pull request descriptions. An
  agent may draft a change; the person who commits it owns it.
- Never commit secrets, real fixtures, `var/`, built binaries or packages.
- Push only when the maintainer asks. The canonical remote is
  `codeberg.org/simplicer/ataecina`; `github.com/simplicer/ataecina` is a
  push mirror.

## Development loop (Ralph Loop)

- **Outer loop**: take the next task of `docs/ROADMAP.md` in order; finish it
  completely before the next; stop only on a real contradiction.
- **Inner loop** per task: place it in its context and layer → write failing
  tests first → implement the minimal correct solution → run `make check`
  and `make e2e` → verify architecture and directory rules → remove dead
  code → refactor safely → re-run everything → update docs (SPECS, ADRs,
  README, CHANGELOG) → apply the version bump → one commit → clean tree.
- Never advance with a failing check. No partial implementations, no TODOs
  left behind without a ROADMAP entry.
- A new architectural decision is a new ADR in `docs/adrs/`, numbered, with
  Context / Decision / Consequences / Alternatives.

## Tooling

- Go-native first: `go build`, `go test`, `go vet`, `gofmt`, `govulncheck`.
  The `Makefile` orchestrates: `make check` (fmt-check, vet, unit,
  coverage of the application packages, integration, acceptance,
  bench-smoke, widget-lint — `qmllint` when installed, `msgfmt --check` —,
  govulncheck, build, widget, check-version), `make e2e`, `make fuzz`,
  `make package` (deb, rpm, Arch and the release archive with `./install`,
  verified by `bin/check-packages`), `bin/publish-release` (tag and
  publish the downloads),
  `make deb` / `make install-deb` (the host's `.deb`, test-installed in a
  fake root, then installed with apt), `make` alone (build and widget),
  `make install-user` (binary, units, widget into `~/.local`). The installed
  layout lives only in `bin/install-files`.
- Builds are reproducible: `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`,
  pinned toolchain, `-ldflags "-s -w -X main.version=$(VERSION)"`.
- Dependencies: the standard library and `github.com/godbus/dbus/v5` only
  (ADR 0009). Build-time tools (nfpm) are pinned and never linked. Adding
  any dependency requires an ADR.
- Executable scripts live in `bin/`; containers, systemd units and packaging
  live in `infra/`; data published for installed copies (the coding plan
  offers, `data/offers.csv`) lives in `data/`.
