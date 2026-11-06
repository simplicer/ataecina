# Roadmap

Ataecina is developed in milestones following the Ralph Loop: every task is
one Conventional Commit; every `feat` raises the minor while the major is 0
(`0.Y.Z-unstable`). Status is kept in sync with `CHANGELOG.md`.

| # | Milestone | Content | Status |
|---|---|---|---|
| 0 | Foundation | Rules (`AGENTS.md`), specification, roadmap, ADRs | done |
| 1 | Core and terminal | `allowances` domain (subscriptions, meters, balances, credits, banked resets, buckets, readings, fidelity, outcomes, freshness, rebirth); snapshot file; `ataecina status` table and `--json` contract; `collect_readings` and `ataecina collect` (concurrent, global deadline, isolated failures, last good reading kept); strict `config.json`; XDG paths and permission checks; Loki-compatible logging; `Makefile`, `bin/set-version`, `bin/check-version` | done |
| 2 | Token-free sources | `codex-logs` (incremental reader, windows by duration), `codex-app-server` fallback (method allowlist), `claude-statusline` hook (chained from an existing status line, or `--print` as the status line itself), `opencode-zen` derived balance and `balance declare` | done |
| 3 | Sealed keys | `keys` context: `init`, sealed `keys.json`, `keys set/list/forget` from stdin without echo; unlockers `secret-service` (D-Bus, ADR 0009) and `passphrase`; `doctor` | done |
| 4 | API-key sources | `openrouter-api`, `vercel-api`, `zai-api`, `commandcode-api`, `minimax-api` (implemented against recorded, synthetic answers; live verification with the owner's keys before release); hardened HTTP client (ADR 0006) | done |
| 5 | Collector service | hardened systemd user service and timer for `ataecina collect`; `systemd-creds` unlocker; `alerts` context with desktop notifications | planned |
| 6 | Plasma widget | `org.simplicer.ataecina` plasmoid (compact ring, full list, refresh through systemd); `qmllint` in `make check` | planned |
| 7 | Packages | `make package` with nfpm (`.deb`, `.rpm`, Arch), AUR `PKGBUILD`, `make install-user`; reproducible builds; signed checksums | planned |
| 8 | Other desktops | `ataecina tray` (StatusNotifierItem + DBusMenu) for XFCE, Cinnamon, MATE, LXQt, Budgie, Waybar and GNOME with AppIndicator | planned |
| 9 | More sources | `alibaba-cli` (`bl usage token-plan`), `ollama-api`, `copilot-gh` (`gh api` billing report), `orcarouter-api`, `cloudflare-api` (each verified live first), `heimdall` observed cooldowns for free tiers | planned |
| 10 | Publication | License, `SECURITY.md`, `CONTRIBUTING.md`, Forgejo CI on Codeberg with GitHub push mirror, first public `0.x` packages | planned |

## Later (not scheduled)

- `outages` context: provider status pages shown as incident badges.
- GNOME Shell extension with the full view (the tray covers GNOME meanwhile).
- Landlock self-sandbox of the collector (Linux ≥ 5.13), restricting reads to
  the documented paths.
- History of readings (daily usage per subscription) kept locally, and token
  history from Codex `account/usage/read` and Claude Code transcripts.
- Waybar and XFCE Generic Monitor output formats.

## Design principles

- The display never touches a key; keys never rest in clear text.
- Prefer what the official tools already write locally over any network call.
- Never a session, a cookie or a login: API keys or official CLIs only.
- One broken provider never breaks the rest.
- One small static binary; the standard library and D-Bus, nothing else.
