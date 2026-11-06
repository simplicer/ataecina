# Ataecina — Business specification

Status: foundation (`0.0.0-unstable`). This document is the source of truth
for behaviour; `AGENTS.md` holds the engineering rules and `docs/adrs/` the
decisions.

## 1. Vocabulary

| Term | Meaning |
|---|---|
| **Subscription** | Something the owner can spend on a provider: a plan with quota windows (Claude Pro, Z.ai Coding Plan), a prepaid balance (OpenRouter, Vercel, OpenCode Zen) or a free tier. Identified by an owner-chosen id (`claude`, `zai`). |
| **Provider** | The company or service behind a subscription (`anthropic`, `openai`, `zai`, `minimax`…). |
| **Allowance** | What a subscription lets the owner spend: a set of meters and/or a balance. |
| **Meter** | One windowed allowance: the window, the percentage used (0–100, may exceed 100 when the provider allows overage), and when it resets. |
| **Window** | The span a meter covers: `FIVE_HOURS`, `DAILY`, `WEEKLY`, `MONTHLY`, `BILLING_CYCLE`, or `OTHER` with its length in minutes. |
| **Balance** | Money left and spent, as a money object (amount + ISO 4217 currency). |
| **Reset (rebirth)** | The instant a window's usage returns to zero. |
| **Reading** | A dated observation of one subscription's allowance from one source, with fidelity and outcome. |
| **Source** | Where a reading comes from: a provider API, an official CLI, a local signal written by an official tool, a declaration by the owner, or Heimdall. |
| **Fidelity** | How trustworthy a figure is: `REPORTED` (the provider says so), `DERIVED` (computed locally from records), `OBSERVED` (inferred from serving traffic through Heimdall), `DECLARED` (typed by the owner). |
| **Freshness** | `FRESH`, `STALE` or `UNKNOWN`, computed from the reading's age (§4.2). |
| **Snapshot** | The latest reading of every configured subscription, written by the collector and read by every display. |
| **Threshold** | A percentage of a meter (default 90 and 95) that raises an alert when crossed. |
| **Alert** | A notification that a meter crossed a threshold, raised once per threshold per window. |
| **Sealed key** | A provider API key encrypted at rest under the master key. |
| **Unlocker** | A way to hold the master key: Secret Service, passphrase, or systemd credential. |
| **Collector** | The one-shot process that reads sources, unseals keys and writes the snapshot. |
| **Display** | Anything that shows the snapshot: terminal table, JSON, Plasma widget, tray. |

## 2. Scope

**In:** reading quota windows and balances of the owner's own AI
subscriptions; keeping the keys needed for that safe; showing the result in a
terminal, as JSON, in KDE Plasma and in a tray; alerting near exhaustion;
installing cleanly on Debian/Ubuntu, Fedora/openSUSE and Arch families.

**Out:** sending prompts or spending quota; logging in to providers, reading
OAuth tokens, cookies or browser storage; scraping web consoles; buying
credits or changing plans; team or multi-user dashboards; Windows and macOS;
pricing comparisons (models.dev may be added later as an explicit feature).

## 3. Sources

Each source is one adapter. Only the hosts below are ever contacted, and only
for subscriptions the owner enabled. **Undocumented** sources must be
re-verified with a live, manual call (owner's key) before an adapter is
merged, and again whenever they report `CONTRACT_CHANGED`. Fields listed are
the only ones read.

| Source id | Subscription | Kind | Host / location | Credential | Fields read | Documented | Fidelity |
|---|---|---|---|---|---|---|---|
| `claude-statusline` | Claude Pro/Max/Ultra | local signal | stdin of `ataecina hook claude-statusline`, called from the Claude Code status line (§3.1) | none | `rate_limits.five_hour` / `seven_day` (and `spend_limit` behind a Claude apps gateway): `used_percentage`, `resets_at` | yes (Claude Code docs) | REPORTED |
| `codex-logs` | ChatGPT / Codex | local signal | `~/.codex/sessions/**/rollout-*.jsonl` and `~/.codex/archived_sessions/` (read-only, incremental) (§3.2) | none | latest `rate_limits` with `limit_id: codex`: windows, credits, plan | no (stable in practice) | REPORTED |
| `codex-app-server` | ChatGPT / Codex | official CLI (opt-in fallback) | `codex app-server` over stdio JSON-RPC, methods `initialize` and `account/rateLimits/read` only (§3.2) | none for Ataecina (Codex uses its own login) | `rateLimitsByLimitId`, `rateLimitResetCredits.availableCount` | experimental protocol, schema published by `codex app-server generate-json-schema` | REPORTED |
| `zai-api` | Z.ai GLM Coding Plan | provider API | `https://api.z.ai/api/monitor/usage/quota/limit` | API key | token limits per window, percentage, next reset | no | REPORTED |
| `minimax-api` | MiniMax Token Plan | provider API | `https://api.minimax.io/v1/token_plan/remains` (or `/coding_plan/remains`, to verify) | API key | interval totals, remaining (field meaning to verify), reset | no | REPORTED |
| `alibaba-cli` | Alibaba Model Studio Token Plan | official CLI | `bl usage token-plan --output json` (executed with a fixed argv, no shell) | `bl`'s own login | quota windows, percentage, reset, `hasQuotaData` | yes (Model Studio docs) | REPORTED |
| `commandcode-api` | Command Code | provider API | `https://api.commandcode.ai` `/alpha/billing/credits`, `/alpha/billing/subscriptions`, `/alpha/usage/summary` | API key | monthly credits, 5-hour and weekly windows, period end | no (used by the official CLI) | REPORTED |
| `openrouter-api` | OpenRouter | provider API | `https://openrouter.ai/api/v1/key`, `/api/v1/credits` | API key (credits may need a management key) | usage, limit, limit remaining, free tier, total credits | yes | REPORTED |
| `vercel-api` | Vercel AI Gateway | provider API | `https://ai-gateway.vercel.sh/v1/credits` | API key | `balance`, `total_used` | yes | REPORTED |
| `opencode-zen` | OpenCode Zen | official CLI + declaration | `opencode stats --days <N> --models --json` (fixed argv, no shell), entries with `model.providerID = "opencode"` | none | per-model `cost` over the range since the last declaration | yes (CLI help) | DERIVED |
| `heimdall` | any plan Heimdall serves | local service | Heimdall admin API on loopback | Heimdall admin token | plan cooldowns, announced resets, usage ledger | yes (Heimdall OpenAPI) | OBSERVED |

### 3.1 Claude in detail (verified 2026-11-06, Claude Code 2.1.319)

- The status line input is the only official source that needs no
  credential. Claude Code has no non-interactive usage command (`/usage`
  exists only inside a session), and the OAuth usage endpoint is excluded by
  ADR 0003.
- `rate_limits` is present only for Pro, Max and Ultra subscribers (or behind
  a Claude apps gateway that sets a spend limit), only after the first API
  response of a session; each window may be absent, and Claude Code drops a
  window once its `resets_at` passes. `used_percentage` is 0–100;
  `resets_at` is Unix epoch seconds.
- The status line runs at session start, after each assistant message, after
  `/compact`, when a window reaches `resets_at`, and every `refreshInterval`
  seconds if configured. Figures change only after an API response, so usage
  made on claude.ai or Claude Desktop (which spends the same windows) is seen
  at the next Claude Code response. Displays therefore show freshness and
  estimate rebirth after `resets_at` (§4.2).
- `ataecina hook claude-statusline` reads the status line JSON from stdin
  (capped at 1 MiB), keeps only `rate_limits`, writes
  `$XDG_RUNTIME_DIR/ataecina/signals/claude.json` atomically and exits 0 in
  a few milliseconds, **even on error**: it must never break or slow the
  owner's status line. It holds no key and opens no socket. The collector
  reads that signal file like any local source.
- With an existing status line script, one line after the script reads its
  input chains the hook:
  `printf '%s' "$input" | ataecina hook claude-statusline &`.
  Without one, `ataecina hook claude-statusline --print` can be the status
  line itself and prints a compact summary (`5h 23% · 7d 41% · ↻ 2h10m`).
- The plan (Pro, Max…) is not in the input: it is declared in `config.json`
  (`"plan": "PRO"`). Ataecina never opens `~/.claude/.credentials.json`,
  because it holds the OAuth token.
- Several Claude accounts (`CLAUDE_CONFIG_DIR`) map to several
  subscriptions: `ataecina hook claude-statusline --subscription claude-work`.

### 3.2 Codex in detail (verified 2026-11-06, codex-cli 0.166.0)

Two official sources; neither gives Ataecina a token. Ataecina never opens
`~/.codex/auth.json`.

**`codex-logs` (default).** After each response Codex writes a
`rate_limits` object into the session log
`~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`. The reader scans only the
newest files, from the last byte offset it processed, and keeps the latest
object with `limit_id == "codex"`:

| Field (snake_case) | Meaning |
|---|---|
| `primary`, `secondary` | windows: `used_percent`, `window_minutes`, `resets_at` (Unix seconds) |
| `credits` | `has_credits`, `unlimited`, `balance` (decimal string) |
| `plan_type` | `free`, `go`, `plus`, `pro`, `team`, `business`, `enterprise`, `edu`… |
| `limit_id`, `limit_name` | bucket (`codex`; also seen: `premium`, `base_model_inference`) |
| `individual_limit`, `spend_control_reached`, `rate_limit_reached_type` | spend controls and which limit was hit |

- Windows are mapped by `window_minutes` (300 → `FIVE_HOURS`, 10080 →
  `WEEKLY`, anything else → `OTHER`), **never by position**: logs where
  `primary` is the weekly window exist (238 of 3 174 entries on the
  reference machine).
- Buckets other than `codex` are shown only when they carry windows.

**`codex-app-server` (opt-in fallback for freshness).** The official binary
is started as `codex app-server` (fixed argv, no shell, stdio, 15 s
timeout). Ataecina sends `initialize` with `clientInfo.name = "ataecina"`,
then `account/rateLimits/read`, reads the answer and closes the process.
Codex performs the network call with its own login.

- Response: `rateLimitsByLimitId` (map by `limit_id`; fallback
  `rateLimits`), each a snapshot with `primary`/`secondary`
  (`usedPercent`, `windowDurationMins`, `resetsAt`), `credits`
  (`hasCredits`, `unlimited`, `balance`), `planType`, `limitId`,
  `limitName`, `spendControlReached`, `rateLimitReachedType`; plus
  `rateLimitResetCredits.availableCount` (banked resets, shown as
  information only).
- Used only when the `codex-logs` reading is older than
  `app_server_after_minutes` (default 60) and the owner enabled it.
- **Method allowlist**: `initialize`, `account/rateLimits/read` (and later
  `account/usage/read` for history). Never `account/rateLimitResetCredit/consume`,
  login, logout, config writes or any method that changes state.
- Codex marks the protocol experimental: any surprise is `CONTRACT_CHANGED`
  and the logs remain the source.

Declared balances (`ataecina balance declare`) are stored in
`$XDG_STATE_HOME/ataecina/balances.json` with fidelity `DECLARED`; `opencode-zen`
derives `remaining = declared top-ups − recorded cost since the declaration`.

Providers with no readable allowance (Gemini AI Studio keys, NVIDIA, Groq,
Kilo, LLM7, Cloudflare) are shown only through `heimdall`, or as
`UNSUPPORTED` with a hint.

## 4. Readings

### 4.1 Reading outcome

Every reading has an outcome: `OK`, `PARTIAL` (some meters missing),
`BROKEN` (the source answered something Ataecina cannot interpret),
`UNAVAILABLE` (network or provider down), `KEY_MISSING`, `LOCKED` (the master
key could not be unlocked), `UNSUPPORTED`.

### 4.2 Freshness and rebirth

- A reading is `FRESH` while its age is under twice the collection interval
  (default interval 5 minutes); `STALE` afterwards; `UNKNOWN` if never read.
- Local signals (Claude, Codex) only change when their tool is used. When the
  clock passes a meter's `resets_at`, the display shows that window as
  reborn — 0 % used, marked `ESTIMATED` — until a new reading arrives.
- A display never hides staleness: every figure carries its `observed_at`.

### 4.3 Error classes

Provider failures are classified in infrastructure:

| Class | Typical cause | Effect |
|---|---|---|
| `KEY_REJECTED` | 401/403, revoked key | outcome `KEY_MISSING`-like hint; not asked again until the key changes |
| `RATE_LIMITED` | 429 | skip the source until `Retry-After` or the next interval |
| `UNAVAILABLE` | timeout, 5xx, DNS | keep the last reading, mark it `STALE` when it ages |
| `CONTRACT_CHANGED` | unexpected shape or fields | outcome `BROKEN`, logged without the body |
| `UNREADABLE` | local file missing, locked or malformed | outcome `UNAVAILABLE` for that source |

## 5. Use cases

| Use case (file) | Context | Actor | Behaviour |
|---|---|---|---|
| `collect_readings` | allowances | collector (timer or `ataecina collect`) | For each enabled subscription, ask its source through the `Source` port (keys via the `KeyRing` port), classify failures, merge with the previous snapshot (keep last good reading on transient errors), write the snapshot atomically. One attempt per source; sources run concurrently with a global deadline. |
| `report_snapshot` | allowances | display | Read the snapshot, apply freshness and rebirth rules at the current time, return the view model. Never touches keys or the network. |
| `record_claude_limits` | allowances | Claude Code status line hook | Parse the status line JSON from stdin, keep only `rate_limits`, write the Claude signal file (§3.1); print nothing, or a summary line with `--print`. Must finish in a few milliseconds and never fail the status line. |
| `declare_balance` | allowances | owner | Record a top-up or current balance for a balance-only subscription. |
| `seal_key` | keys | owner (`keys set`) | Read the key from stdin, seal it under the master key with the provider id as associated data, write `keys.json` atomically. |
| `forget_key` | keys | owner | Remove a sealed key. |
| `list_keys` | keys | owner | List providers that have a sealed key, with the date set; never the key. |
| `unseal_key` | keys | collector, through the `KeyRing` port | Unlock the master key with the first available unlocker and open one key. |
| `add_unlocker` / `remove_unlocker` | keys | owner | Wrap the master key for a new unlocker / drop one; at least one must remain. |
| `raise_threshold_alerts` | alerts | collector | Compare new meters with thresholds; raise each crossed threshold once per window; forget alert state when the window is reborn. |

## 6. Keys

- `keys.json` (`0600`):
  `{ "version": 1, "master_key": { "<unlocker>": <wrapped key> … }, "keys": { "<provider>": { "nonce": "<base64>", "sealed": "<base64>", "set_at": "<RFC 3339>" } } }`.
- Sealing: AES-256-GCM, 96-bit random nonce, associated data
  `ataecina:v1:<provider>` so a sealed key cannot be moved to another
  provider.
- Master key: 32 random bytes from `crypto/rand`, created by `ataecina init`.
- Unlockers:
  - `secret-service`: the master key is an item in the default collection of
    the session's `org.freedesktop.secrets` provider (KWallet/ksecretd on
    Plasma, GNOME Keyring elsewhere), attributes
    `{ application: ataecina, purpose: master-key }`. Works from any terminal
    inside the desktop session; may show the keyring's own unlock prompt.
  - `passphrase`: master key wrapped with AES-256-GCM under a key derived by
    PBKDF2-HMAC-SHA256 (≥ 600 000 iterations, 16-byte salt). Prompted on a
    terminal without echo. For SSH, text consoles and machines without a
    keyring.
  - `systemd-creds`: master key encrypted with `systemd-creds encrypt --user`
    (TPM2 when present) and given to the collector unit through
    `LoadCredentialEncrypted=`. For headless collectors.
- `doctor` reports which process owns the Secret Service name, which unlockers
  exist and whether file permissions are correct.

## 7. Alerts

- Default thresholds: 90 and 95 % of any meter; configurable per subscription.
- Delivered through `org.freedesktop.Notifications` (Plasma, GNOME, XFCE…);
  the Plasma widget also highlights the meter.
- State in `$XDG_STATE_HOME/ataecina/alerts.json`, keyed by subscription,
  window and `resets_at`, so each threshold fires once per window.

## 8. Displays and the JSON contract

### 8.1 Terminal

`ataecina` prints one line per subscription: id, worst meter, every window as
`used% · resets in Xh Ym`, balance, freshness. Colours only on a TTY.

### 8.2 JSON (`ataecina status --json`)

Public, versioned contract (Zalando JSON rules). Example:

```json
{
  "data": {
    "subscriptions": [
      {
        "id": "claude",
        "provider": "anthropic",
        "plan": "PRO",
        "outcome": "OK",
        "fidelity": "REPORTED",
        "freshness": "FRESH",
        "source": "CLAUDE_STATUSLINE",
        "observed_at": "2026-11-06T19:40:00Z",
        "meters": [
          { "window": "FIVE_HOURS", "used_percent": 23.5, "resets_at": "2026-11-06T22:00:00Z", "estimated": false },
          { "window": "WEEKLY", "used_percent": 41.2, "resets_at": "2026-11-11T08:00:00Z", "estimated": false }
        ],
        "balance": null
      },
      {
        "id": "openrouter",
        "provider": "openrouter",
        "plan": null,
        "outcome": "OK",
        "fidelity": "REPORTED",
        "freshness": "FRESH",
        "source": "OPENROUTER_API",
        "observed_at": "2026-11-06T19:40:02Z",
        "meters": [],
        "balance": { "remaining": { "amount": 8.42, "currency": "USD" }, "spent": { "amount": 11.58, "currency": "USD" } }
      }
    ]
  },
  "meta": { "version": "0.1.0-unstable", "generated_at": "2026-11-06T19:41:00Z", "worst_used_percent": 41.2 }
}
```

Errors are RFC 9457 problem objects on stderr with a non-zero exit code.

### 8.3 Plasma widget

A native Plasma 6 widget (`org.simplicer.ataecina`) built with Plasma and
Kirigami components, so it follows the Breeze theme, light and dark colour
schemes and the accent colour like the battery or network widgets.

- **Compact form (panel):** a ring drawn around the Ataecina icon showing
  the worst meter, optionally with its percentage. Colours: normal; warning
  from the first threshold; critical from the second; grey when the figure is
  stale or broken. Tooltip: worst subscription, window and time to rebirth.
- **Full form (popup):** one block per subscription with its plan, a bar per
  window, the percentage, the countdown to rebirth (`↻ 2 h 10 m`), the
  balance when there is one, a fidelity badge for anything not `REPORTED`,
  and the outcome when not `OK`. A footer shows when the snapshot was taken
  and a refresh button.

```text
┌──────────────────────────────────────────────────┐
│  Ataecina                       updated 2 min ago ⟳ │
├──────────────────────────────────────────────────┤
│  Claude · Pro                                    │
│  5 h    ███████░░░░░░░░░░░░░  23 %   ↻ 2 h 10 m  │
│  week   ████████░░░░░░░░░░░░  41 %   ↻ 4 d 12 h  │
│  Codex · Plus                                    │
│  5 h    █░░░░░░░░░░░░░░░░░░░   4 %   ↻ 3 h 02 m  │
│  week   ░░░░░░░░░░░░░░░░░░░░   1 %   ↻ 6 d 01 h  │
│  Z.ai GLM · Lite                                 │
│  5 h    ██████████████████░░  92 % ⚠ ↻ 0 h 41 m  │
│  OpenRouter              $8.42 left · $11.58 used │
│  OpenCode Zen            ≈ $3.10 left   derived  │
│  MiniMax                 ✕ contract changed      │
└──────────────────────────────────────────────────┘
```

- It runs `ataecina status --json` on a timer (default 60 s) through the
  Plasma 5 support executable engine; "refresh now" starts
  `ataecina-collect.service` through systemd. It never collects, never reads
  keys and has no network code.
- Settings page: which subscriptions to show and in which order, show the
  percentage in the panel, widget refresh interval.
- Translatable through KDE i18n: English source strings, Spanish shipped from
  the first release.

### 8.4 Tray (`ataecina tray`)

StatusNotifierItem with the same icon and a menu listing every subscription;
for XFCE, Cinnamon, MATE, LXQt, Budgie, Waybar and GNOME with the AppIndicator
extension.

## 9. Files and paths

| Path | Content | Mode |
|---|---|---|
| `$XDG_CONFIG_HOME/ataecina/` | configuration directory | `0700` |
| `…/config.json` | subscriptions, sources, thresholds, interval (no secrets) | `0600` |
| `…/keys.json` | sealed keys and wrapped master key | `0600` |
| `$XDG_RUNTIME_DIR/ataecina/snapshot.json` | latest readings (figures only) | `0600` |
| `$XDG_RUNTIME_DIR/ataecina/signals/` | signals written by hooks (Claude limits), read by the collector | `0700` / `0600` |
| `$XDG_STATE_HOME/ataecina/` | alert state, declared balances, log offsets, log | `0700` / `0600` |

`config.json` example:

```json
{
  "version": 1,
  "interval_minutes": 5,
  "alerts": { "thresholds": [90, 95] },
  "subscriptions": [
    { "id": "claude", "source": "claude-statusline", "plan": "PRO" },
    { "id": "codex", "source": "codex-logs", "app_server_after_minutes": 60 },
    { "id": "zai", "source": "zai-api", "key": "zai" },
    { "id": "openrouter", "source": "openrouter-api", "key": "openrouter" },
    { "id": "zen", "source": "opencode-zen" }
  ]
}
```

## 10. Constraints

- Display path: no network, no decryption, < 50 ms.
- Collector: one-shot, bounded by a global deadline (default 20 s), memory
  well under 30 MB.
- Single static binary (`CGO_ENABLED=0`); runtime dependencies: none beyond
  the session D-Bus.
- Linux only. Plasma 6 is the reference desktop; others through the tray.
