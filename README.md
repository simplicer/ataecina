# Ataecina

> Every AI subscription you pay for, how much is left and when it is reborn —
> without handing your sessions to anyone.

Ataecina is a small Go program for Linux that shows, at a glance, the state of
each AI subscription and balance you use — Claude, ChatGPT/Codex, Z.ai GLM,
MiniMax, Alibaba Model Studio, Command Code, OpenRouter, Vercel AI Gateway,
OpenCode Zen and more — and when each quota window resets. It works as a
command-line tool, as a **KDE Plasma widget** and as a tray icon for other
desktops.

**Current version:** `0.2.0-unstable` — early development: the core, the
terminal table and the JSON contract work; sources arrive milestone by
milestone (see [docs/ROADMAP.md](docs/ROADMAP.md)).

## Why "Ataecina"

Ataecina (also written *Ataegina* or *Adaegina*) was a goddess of the
pre-Roman peoples of the west of the Iberian Peninsula. Her main sanctuary
stood at Turobriga, in today's province of Huelva, and altars to her have been
found in Mérida and at Santa Lucía del Trampal, in Cáceres. She was the
goddess of spring and of rebirth — of what fades and comes back. The Romans
identified her with Proserpina, who spends part of the year in the underworld
and returns every spring. Her name is usually read as the Celtic *ate-gena*,
"the reborn".

That is exactly the life of an AI quota: you spend it, it runs out, and it is
reborn at the next reset — every five hours, every week, every month.
Ataecina watches each of those cycles and tells you how much is left and when
it will be reborn.

The name is also short, easy to type, and keeps company with its sibling
project [Heimdall](https://codeberg.org/simplicer/heimdall), the model relay
that spends those quotas in the right order.

## What it shows

For each subscription you configure:

- **Meters** — the percentage used of each window (5 hours, weekly, monthly)
  and the exact time it resets.
- **Balances** — prepaid credit left and spent so far.
- **Fidelity** — whether the figure is *reported* by the provider, *derived*
  locally, *observed* by Heimdall or *declared* by you.
- **Freshness** — when it was read, so a stale figure never looks current.
- **Alerts** — a desktop notification when a meter crosses 90 % and 95 %
  (configurable), once per window.

## Supported subscriptions (planned)

| Subscription | What Ataecina reads | Source | Credential | Fidelity |
|---|---|---|---|---|
| Claude (Pro, Max) | 5-hour and weekly usage, resets | Claude Code status line (`ataecina hook claude-statusline`) | none | reported |
| ChatGPT / Codex | 5-hour and weekly usage, credits, plan, banked resets | Codex session logs in `~/.codex/sessions`; the official `codex app-server` when they are stale (opt-in) | none | reported |
| Z.ai GLM Coding Plan | 5-hour and weekly token quotas | Z.ai quota API (undocumented) | API key | reported |
| MiniMax Token Plan | interval quota, reset | MiniMax `remains` API (undocumented) | API key | reported |
| Alibaba Model Studio Token Plan | quota usage, reset | official `bl usage token-plan` CLI | `bl`'s own login | reported |
| Command Code | monthly credits, 5-hour and weekly windows | Command Code account API used by its own CLI (alpha) | API key | reported |
| OpenRouter | credits, key limit, free tier | OpenRouter API | API key | reported |
| Vercel AI Gateway | balance and total spent | AI Gateway `/v1/credits` | API key | reported |
| OpenCode Zen | balance | your declared top-ups minus the cost reported by `opencode stats` | none | derived |
| Ollama Cloud | session and weekly usage (or monthly credits on newer plans) | ollama.com usage API (undocumented) | API key | reported |
| GitHub Copilot | premium requests or AI credits used this month | official `gh api` against GitHub's billing usage report, with your monthly allowance declared | `gh`'s own login | derived |
| Orca Router | remaining allowance and usage | OrcaRouter billing API | API key | reported |
| Cloudflare Workers AI | neurons used today out of the free daily allocation | Cloudflare GraphQL Analytics API | read-only analytics token | reported |
| Gemini (AI Studio keys), NVIDIA, Groq, Kilo Gateway, LLM7 | cooldowns and resets seen while serving | [Heimdall](https://codeberg.org/simplicer/heimdall), optional | Heimdall admin token | observed |
| Gemini CLI (Google account), Qwen Code (Qwen OAuth) | nothing readable without the tool's login token | — | — | unsupported |

Available in this version: **ChatGPT / Codex** from its session logs. The
other sources arrive as listed in [docs/ROADMAP.md](docs/ROADMAP.md) and read
as `UNSUPPORTED` until then.

Undocumented endpoints may change without notice. When one does, only that
subscription shows `BROKEN`; everything else keeps working. Adding a provider
is one small adapter (see [AGENTS.md](AGENTS.md#adding-a-provider-the-common-task)).

### Claude Code

Claude Code hands its rate limits to your status line command. Add one line
to your status line script, right after it reads its input:

```sh
printf '%s' "$input" | ataecina hook claude-statusline &
```

or, if you have no status line yet, let Ataecina be it:

```json
{ "statusLine": { "type": "command", "command": "ataecina hook claude-statusline --print" } }
```

Ataecina never opens `~/.claude/.credentials.json` or `~/.codex/auth.json`.

## One program, one widget

Everything is **one binary**, `ataecina`, with subcommands: the timer runs
`ataecina collect`, you type `ataecina` in a terminal, and on non-Plasma
desktops `ataecina tray` is the tray icon. On **KDE Plasma** the panel icon
and its popup are a native Plasma widget — a few QML files that Plasma itself
draws, themed like the battery or network widgets — which asks
`ataecina status --json` for the figures. The same package installs both.

## How your keys are kept safe

Ataecina is built on one idea: **the program that draws the numbers never
touches a key, and the keys never rest in clear text.**

- **No sessions, ever.** Ataecina never reads OAuth tokens, browser cookies or
  web consoles, and never logs in on your behalf. Claude and Codex are read
  from what their own tools already write on your machine; everything else
  uses an API key you give it, or the provider's official CLI.
- **Sealed keys.** Your keys live in `~/.config/ataecina/keys.json`, but each
  one is encrypted (AES-256-GCM) with a random master key. Copying that file
  reveals nothing.
- **Unlockers** hold the master key:
  - **Secret Service** — KWallet on Plasma, GNOME Keyring elsewhere; unlocked
    when you log in, so nothing is asked.
  - **Passphrase** — for consoles, SSH sessions and machines without a
    desktop keyring.
  - **systemd credentials** — bound to this machine's TPM2, for the
    background collector on headless systems.
- **Two halves.** A one-shot collector, run every few minutes by a hardened
  systemd user timer, is the only part that unseals keys and talks to the
  network. The terminal view, the Plasma widget and the tray only read a
  small snapshot of figures kept in memory (`$XDG_RUNTIME_DIR`).
- **Short leash on the network.** Each provider adapter can only reach its
  own HTTPS host, never follows redirects, caps every answer and stops when
  the provider says stop. No telemetry, no update checks.
- **Strict files.** Ataecina refuses to start if its configuration or key
  file can be read by anyone else, like ssh does.

### Using it from a terminal

In a terminal inside your desktop session (Konsole, GNOME Terminal…) the
command talks to the Secret Service over the session D-Bus — the same channel
KWallet uses — so it gets the master key without any window. Over SSH or on a
text console there is no desktop session, so you unlock with your passphrase
(or rely on the systemd credential for the background collector).

## Install (planned)

| Distribution | Package |
|---|---|
| Debian, Ubuntu, KDE neon, Kubuntu | `.deb` |
| Fedora, openSUSE | `.rpm` |
| Arch, Manjaro, EndeavourOS | Arch package and an AUR `PKGBUILD` |
| Anything else | static binary, or `make install-user` from source |

Packages install the `ataecina` binary, the collector's systemd user timer,
the Plasma widget and the tray launcher. Then:

```sh
ataecina init                      # create ~/.config/ataecina with safe permissions
ataecina keys set zai              # paste the key; it is read without echo and sealed
systemctl --user enable --now ataecina-collect.timer
ataecina                           # the table
```

Add the **Ataecina** widget to your Plasma panel. On XFCE, Cinnamon, MATE,
LXQt or GNOME (with the AppIndicator extension) start `ataecina tray`.

## Commands

```text
ataecina [status] [--json]          show the snapshot (no network, no keys)
ataecina collect                    read every subscription now (what the timer runs)
ataecina version
ataecina help
```

Planned:

```text
ataecina keys set|list|forget       manage sealed keys (set reads stdin)
ataecina unlockers add|list|remove  manage how the master key is held
ataecina balance declare            record a top-up for balance-only providers
ataecina hook claude-statusline     record Claude limits from the Claude Code status line
ataecina tray                       tray icon for non-Plasma desktops
ataecina doctor                     check permissions, keyring, units and sources
```

### Configuration today

Until `ataecina init` exists (milestone 3), create the configuration by hand.
Ataecina refuses to run if the directory is not `0700` or the file not `0600`:

```sh
install -d -m 700 ~/.config/ataecina
install -m 600 etc/ataecina.example.json ~/.config/ataecina/config.json
$EDITOR ~/.config/ataecina/config.json   # keep only your subscriptions
ataecina collect && ataecina
```

Sources that this version cannot read yet show as `UNSUPPORTED`; nothing
breaks. The options of each source are listed in
[docs/SPECS.md](docs/SPECS.md) §9.

## Architecture

Domain-Driven Design with vertical slices and Screaming Architecture, in the
Go layout: production code under `internal/<context>/{domain,application,infrastructure}`,
the composition root in `cmd/ataecina/`.

| Context | Purpose |
|---|---|
| `allowances` | subscriptions, meters, balances, readings and the snapshot |
| `keys` | sealed provider keys and their unlockers |
| `alerts` | thresholds and notifications |
| `outages` *(planned)* | provider status pages |

The Plasma widget lives in `share/plasma/plasmoids/org.simplicer.ataecina/`,
systemd units in `infra/systemd/`, packaging in `infra/packaging/`. Rules are
in [AGENTS.md](AGENTS.md), the specification in [docs/SPECS.md](docs/SPECS.md)
and decisions in [docs/adrs/](docs/adrs/).

## Development

Go 1.27 or later; nothing else is needed (`govulncheck` is fetched at a
pinned version by `make check`).

```sh
make check      # format, vet, unit (100 % of use cases), integration, acceptance, benchmarks, vulnerabilities, build, version
make e2e        # the built binary end to end against fake providers
make fuzz       # explore the decoders further (FUZZ_TIME=30s)
make build      # var/build/ataecina, static and reproducible
```

`make package` (`.deb`, `.rpm`, Arch) arrives with milestone 7. The suite
never calls a real provider and never touches your real keys or your home:
every test runs in a throw-away `HOME` and XDG tree.

## Versioning

Semantic Versioning with the Simplicer policy: `VERSION` is the single source
of truth; while the major is 0 every version is `0.Y.Z-unstable`; `feat`
raises the minor, `fix`/`refactor`/`perf` the patch; releases are annotated
tags on frozen `release/X.Y.Z` branches. See [CHANGELOG.md](CHANGELOG.md).

## License

To be decided before the first public push.
