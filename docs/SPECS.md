# Ataecina — Business specification

Status: see `docs/ROADMAP.md`. This document is the source of truth
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

Sources are chosen in this order of preference (ADR 0003): a local signal
written by the official client → the official CLI run with a fixed argv →
a documented API with the owner's API key → an undocumented API with the
owner's API key → what Heimdall observes while serving → `UNSUPPORTED`.
"Verified" records how each row was checked: `local` (the tool's own files
or `--help` on the reference machine), `docs` (the provider's published
documentation) or `third-party` (issues or write-ups of other clients);
**to verify** marks anything not yet confirmed by a live, manual call.

| Source id | Subscription | Kind | Host / location | Credential | Fields read | Documented | Fidelity | Verified |
|---|---|---|---|---|---|---|---|---|
| `claude-statusline` | Claude Pro/Max/Ultra | local signal | stdin of `ataecina hook claude-statusline`, called from the Claude Code status line (§3.1) | none | `rate_limits.five_hour` / `seven_day` (and `spend_limit` behind a Claude apps gateway): `used_percentage`, `resets_at` | yes (Claude Code docs) | REPORTED | local 2026-11-06 |
| `codex-logs` | ChatGPT / Codex | local signal | `~/.codex/sessions/**/rollout-*.jsonl` and `~/.codex/archived_sessions/` (read-only, incremental) (§3.2) | none | latest `rate_limits` with `limit_id: codex`: windows, credits, plan | no (stable in practice) | REPORTED | local 2026-11-06 |
| `codex-app-server` | ChatGPT / Codex | official CLI (opt-in fallback) | `codex app-server` over stdio JSON-RPC, methods `initialize` and `account/rateLimits/read` only (§3.2) | none for Ataecina (Codex uses its own login) | `rateLimitsByLimitId`, `rateLimitResetCredits.availableCount` | experimental protocol, schema published by `codex app-server generate-json-schema` | REPORTED | local 2026-11-06; live to verify |
| `zai-api` | Z.ai GLM Coding Plan | provider API | `https://api.z.ai/api/monitor/usage/quota/limit` (§3.5) | API key (`Bearer`, to verify) | `data.limits[]` of type `TOKENS_LIMIT`/`CREDIT_LIMIT`: `unit`, `number`, `percentage`, `nextResetTime`; `data.level` | no | REPORTED | third-party 2026-11-06; live to verify |
| `minimax-api` | MiniMax Token Plan | provider API | `https://api.minimax.io/v1/token_plan/remains` (§3.5) | Token Plan key (`Bearer`) | `model_remains[]`: interval and weekly totals, counts (remaining), `*_remaining_percent`, start/end times; `base_resp.status_code` | no | REPORTED | third-party 2026-11-06; live to verify |
| `alibaba-cli` | Alibaba Model Studio Token Plan | official CLI | `bl usage token-plan --output json` (executed with a fixed argv, no shell) | `bl`'s own login | quota windows, percentage, reset, `hasQuotaData` | yes (Model Studio docs) | REPORTED | docs 2026-11-06; `bl` not installed, to verify |
| `commandcode-api` | Command Code | provider API | `https://api.commandcode.ai` `/alpha/billing/credits`, `/alpha/usage/summary`, `/alpha/billing/subscriptions` (§3.5) | API key (`Bearer`) | `credits.{monthly,purchased,free}Credits`, `windowLimits.{fiveHour,weekly}.{used,cap,resetAt}`, `totalCost`, `data.{planId,currentPeriodEnd}` | no (used by the official CLI) | REPORTED | third-party 2026-11-06; live to verify |
| `openrouter-api` | OpenRouter | provider API | `https://openrouter.ai/api/v1/key`, `/api/v1/credits` (§3.5) | API key (`Bearer`; `/credits` may need a management key) | `data.{usage,limit,limit_remaining,free_model_daily_requests}`, `data.{total_credits,total_usage}` | yes | REPORTED | docs 2026-11-06 |
| `vercel-api` | Vercel AI Gateway | provider API | `https://ai-gateway.vercel.sh/v1/credits` (§3.5) | API key (`Bearer`) | `balance`, `total_used` (USD decimal strings) | yes | REPORTED | docs 2026-11-06 |
| `opencode-zen` | OpenCode Zen | official CLI + declaration | `opencode stats --all --models --limit 1000 --json --standalone` (fixed argv, no shell), entries with `model.providerID = "opencode"` | none | per-model `cost` to date, against the declared balance (§3.4) | yes (CLI help) | DERIVED | local 2026-11-06 (opencode 2.2.5 `--help`); JSON shape and `--standalone` to verify |
| `ollama-api` | Ollama Cloud | provider API | `https://ollama.com/api/usage` (§3.3) | Ollama API key (`Authorization: Bearer`) | `limits.session.usage`, `limits.weekly.usage` (fraction 0–1 of the cap); monthly-credit plans: fields to verify | no (called by the ollama.com web page) | REPORTED | third-party 2026-11-06; live to verify |
| `copilot-gh` | GitHub Copilot | official CLI | `gh api` (fixed argv, no shell) → `https://api.github.com/users/<login>/settings/billing/premium_request/usage?year=<Y>&month=<M>`, or the AI-credit usage report for a user (same schema, path to verify) (§3.3) | `gh`'s own login, which needs the "Plan" read permission (scope to verify) | `usageItems[]` with `product` Copilot: `grossQuantity`, `discountQuantity`, `netQuantity`, `netAmount`; monthly allowance declared in `config.json` | yes (GitHub REST API `2026-03-10`) | DERIVED | docs 2026-11-06; live to verify |
| `orcarouter-api` | Orca Router | provider API | `https://api.orcarouter.ai/v1/dashboard/billing/subscription`, `/v1/dashboard/billing/usage` | API key (`sk-orca-…`) | remaining allowance and expiry; cumulative usage (`TotalUsage`; JSON names and units to verify) | yes (OrcaRouter "Billing & usage") | REPORTED | docs 2026-11-06; live to verify |
| `cloudflare-api` | Cloudflare Workers AI | provider API | `https://api.cloudflare.com/client/v4/graphql` (GraphQL Analytics, one fixed query) (§3.3) | API token with *Account Analytics: Read* only, plus the account id | `aiInferenceAdaptiveGroups` → `sum.totalNeurons` for the current UTC day, against the published 10 000 neurons/day allocation | yes (GraphQL Analytics API; dataset fields to verify) | REPORTED | docs and third-party 2026-11-06; live to verify |
| `heimdall` | any plan Heimdall serves | local service | Heimdall admin API on loopback | Heimdall admin token | plan cooldowns, announced resets, usage ledger | yes (Heimdall OpenAPI) | OBSERVED | to verify |

### Providers without a readable allowance

| Provider | What exists (2026-11-06) | Decision |
|---|---|---|
| Google AI Studio (Gemini API keys) | Limits are per Google Cloud project, not per key; requests per day reset at midnight Pacific time (docs). Usage is shown only in the AI Studio console; reading Cloud quota metrics needs a Google OAuth or service-account credential. | `heimdall` (`OBSERVED`: `429 RESOURCE_EXHAUSTED`, daily rebirth at midnight Pacific), else `UNSUPPORTED` |
| Gemini CLI with a Google account | Quota is shown only inside a session (`/stats`); the CLI has no non-interactive usage command (`gemini --help`, 0.1.9) and its quota call needs the Google OAuth token (ADR 0003). | `UNSUPPORTED`. Candidate, to verify: a `DERIVED` request count from the CLI's opt-in local telemetry file (`telemetry.target: local`, prompts not logged), read for event names only |
| Qwen Code (Qwen OAuth) | The free Qwen OAuth tier was discontinued on 2026-04-15 (`qwen auth` reports "removed" in 0.21.8). Qwen Code on a Model Studio plan is covered by `alibaba-cli`. | `UNSUPPORTED` |
| NVIDIA (build.nvidia.com) | Trial credits and a per-minute rate limit, visible only in the console; no usage API. | `heimdall` (`OBSERVED`), else `UNSUPPORTED` |
| Groq | Documented `x-ratelimit-limit/remaining/reset-requests` (per day) and `-tokens` (per minute) headers on every inference response; no endpoint answers without spending a request. | `heimdall` (`OBSERVED` from the headers of traffic it already serves) |
| LLM7 | Free-token quotas (tokens per day, requests per second/minute/hour) that may change without notice; only a models catalogue is documented. | `heimdall` (`OBSERVED`), else `UNSUPPORTED` |
| Kilo Gateway | The documented gateway API has chat, FIM, models and providers only; the balance is shown in the Kilo app, whose profile call uses the app's device-login token (ADR 0003). Free models: 200 requests/hour per IP. | `heimdall` (`OBSERVED`: `402` balance, `429` free models), else `UNSUPPORTED` |

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
- Mapping: `five_hour` → `FIVE_HOURS`, `seven_day` → `WEEKLY`, both in the
  main bucket; `spend_limit` → bucket `spend_limit`, window `BILLING_CYCLE`
  (to verify against a gateway). `resets_at` must be Unix seconds between
  2020 and 2100; anything else, or a missing `used_percentage`, makes the
  whole input malformed. Malformed or oversized input records nothing.
- **Replays never rewind.** Every Claude Code session re-draws its status
  line, and an idle one keeps passing the figures of its last API response.
  The hook therefore merges each input with the signal already kept: per
  window, a later `resets_at` (by more than a minute) is a newer window and
  wins; within the same window usage never decreases, so the higher
  percentage wins; an earlier `resets_at` is a replay and loses. Windows
  the input lacks are kept while still open. The signal (and its
  `observed_at`) is rewritten only when the input brought something newer,
  so freshness means "the last time Claude Code reported a change".
- The signal is `{"version": 1, "reading": {…}}` with the reading fields of
  §8.2; nothing else from the status line input (session, paths, cost,
  model) is kept. `--print` shows every window with its whole percentage
  and the time to the nearest rebirth, or `5h – · 7d –` before the first
  figures.
- The hook reads no configuration and writes nothing else; every failure
  (no `XDG_RUNTIME_DIR`, bad flags, an invalid `--subscription`) goes to
  stderr and the exit code is still 0.
- The collector reads the signal like any source; without one the
  subscription is `UNAVAILABLE` (`UNREADABLE`) with a hint to add the hook.

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
- Only lines of type `event_msg` whose `payload.type` is `token_count` are
  decoded, and of them only `timestamp` and `payload.rate_limits`; a line is
  parsed at all only if it contains `"rate_limits"`. Lines longer than
  64 KiB are skipped unread; a last line without its newline is left for
  the next collection. A missing `limit_id` means `codex`; older logs with
  `resets_in_seconds` are read relative to the event's `timestamp`.
- Files are walked under `$CODEX_HOME` (when absolute, as Codex reads it)
  or `~/.codex`, without following symlinks. Files modified within the last
  8 days (a weekly window plus a day) are read incrementally: for each the
  reader keeps its inode and byte offset, plus the latest figures per
  `limit_id`, in `$XDG_STATE_HOME/ataecina/codex-logs/<subscription>.json`
  (`0600`); no log content is ever stored. A rotated or truncated file is
  read again from the start.
- Codex sometimes logs its own bucket with `primary`, `secondary` and
  `credits` set to null. A null is "unknown", never "gone": per `limit_id`
  every field (windows, credits, plan) keeps its most recent known value,
  by `timestamp`, not by file order, and `observed_at` follows the event
  the windows come from.
- When the recent files give no window for the `codex` bucket (Codex left
  unused for more than 8 days), older files are read **once**, newest
  first, until they do (at most 256 MiB); the result is kept in the state,
  so a Codex unused for weeks still shows its plan and its windows, reborn
  by then, marked stale.
- Failures: no Codex directory, or no rate limits in recent logs →
  `UNREADABLE` (outcome `UNAVAILABLE`, the last good reading is kept);
  only malformed `rate_limits` → `CONTRACT_CHANGED` (`BROKEN`); the
  collector's deadline → `UNAVAILABLE`.

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
- **Method allowlist**, enforced in code before anything is written to the
  process: the `initialize` request, the `initialized` notification that the
  protocol requires right after it (an acknowledgement; it reads and changes
  nothing), and `account/rateLimits/read` (later `account/usage/read` for
  history). Never `account/rateLimitResetCredit/consume`, login, logout,
  config writes or any method that changes state. Server notifications and
  server requests are read and ignored; Ataecina never answers them.
- Codex marks the protocol experimental: any surprise is `CONTRACT_CHANGED`
  and the logs remain the source.
- `codex` is looked up in the absolute directories of the collector's
  `PATH` (relative entries are ignored) and started with the collector's
  environment, so it finds its own login; its stderr is discarded. Messages
  are JSONL; a line over 1 MiB is skipped and a session reads at most
  4 MiB. After the answer stdin is closed and the process gets `SIGTERM`
  (`SIGKILL` 2 s later); the whole session is bounded by 15 s and by the
  collector's deadline.
- Failures: `codex` not on `PATH` → `UNREADABLE`; no answer, or a JSON-RPC
  error (for example Codex not logged in) → `UNAVAILABLE`; an unknown
  method (`-32601`) or an unexpected result → `CONTRACT_CHANGED`. With
  `app_server: true` on a `codex-logs` subscription, the logs are read
  first; the app server is asked only when they fail or are older than
  `app_server_after_minutes`, and whenever it fails the logs' reading (or
  their failure) stands. A `codex-app-server` subscription asks the app
  server alone. The reading's `source` says which one answered.

### 3.3 Other sources in detail (researched 2026-11-06, not yet verified live)

**`ollama-api` (Ollama Cloud).** The `ollama` CLI (0.31.1) has no usage
command. The ollama.com usage page reads `GET https://ollama.com/api/usage`,
which accepts the owner's Ollama API key as `Authorization: Bearer` (an
`X-API-Key` header is rejected). It returns `limits.session.usage` and
`limits.weekly.usage` as a fraction of the cap (0–1, shown ×100) and no exact
reset instant, so those meters are `FIVE_HOURS` and `WEEKLY` with an unknown
`resets_at`. Plans started after 2026-08-31 replace both windows with a
monthly credit allowance that renews on the subscription day; its fields
are to verify. The same key can spend credits, so it is sealed like any
other key.

**`copilot-gh` (GitHub Copilot).** The Copilot CLI shows usage only inside a
session (`/usage`). GitHub publishes a per-user billing report:
`GET /users/<login>/settings/billing/premium_request/usage` (query `year`,
`month`, optional `day`, `model`, `product`; header
`X-GitHub-Api-Version: 2026-03-10`), whose `usageItems[]` carry `product`,
`sku`, `model`, `unitType`, `grossQuantity`, `discountQuantity`,
`netQuantity` and `netAmount`. Since 2026-06-01 Copilot plans are billed in
AI credits (premium requests remain only for annual Pro/Pro+ plans that
stayed on them); GitHub documents an AI-credit report for a user with the
same schema (path to verify). Ataecina runs the official `gh` with a fixed
argv (`gh api --method GET -H X-GitHub-Api-Version:2026-03-10 <path>`), so
`gh` uses its own login and Ataecina never sees a token; the login name and
the monthly allowance are declared in `config.json`. Used percentage =
`Σ grossQuantity` ÷ declared allowance (`DERIVED`); the `MONTHLY` window is
reborn on the 1st at 00:00 UTC (documented). The user report only covers
plans the owner pays personally.

**`orcarouter-api` (Orca Router).** OrcaRouter documents two summary
endpoints authenticated with the API key: `/v1/dashboard/billing/subscription`
(remaining allowance and expiry) and `/v1/dashboard/billing/usage`
(cumulative usage). Rate limits are per workspace and only `Retry-After` is
returned. Key limits may renew daily, weekly or monthly (UTC); there is no
documented call to read a key's own limit.

**`cloudflare-api` (Cloudflare Workers AI).** Workers AI grants 10 000
neurons per day on every plan. The GraphQL Analytics API
(`POST https://api.cloudflare.com/client/v4/graphql`) exposes the
`aiInferenceAdaptiveGroups` dataset; Ataecina sends one fixed query
filtered by `accountTag` and today's UTC date and reads `sum.totalNeurons`.
The token needs only *Account Analytics: Read* (it cannot run inference).
The `DAILY` window is reborn at 00:00 UTC (to verify); analytics may lag a
few minutes behind the dashboard.

### 3.5 API-key sources in detail (milestone 4; undocumented ones verified live by the owner before release)

Every request goes through the hardened client of ADR 0006 with the
subscription's sealed key (`key`, default the subscription id) as
`Authorization: Bearer <key>`; the key is unsealed in memory, used once and
wiped.

- **`openrouter-api`.** `/api/v1/key` gives the key's `usage` (USD) and its
  optional `limit`/`limit_remaining`; `/api/v1/credits` gives the account's
  `total_credits` and `total_usage`. Remaining is `total_credits −
  total_usage` when `/credits` answers, else the key's `limit_remaining`;
  without either the reading is `PARTIAL` with the spend only. Free-model
  requests (`free_model_daily_requests.used/limit`) become a `DAILY` meter in
  bucket `free_models`, reborn at 00:00 UTC.
- **`vercel-api`.** `balance` and `total_used` (USD strings) are remaining
  and spent.
- **`zai-api`.** Entries of `data.limits` with type `TOKENS_LIMIT` or
  `CREDIT_LIMIT` (renamed by Z.ai in November 2026) are windows: `unit` 3 is
  hours (`number` 5 → `FIVE_HOURS`), `unit` 6 is weeks (`number` 1 →
  `WEEKLY`); `percentage` is the share used; `nextResetTime` is epoch
  milliseconds and absent for an untouched window. `TIME_LIMIT` entries
  (tool calls) are skipped. `data.level` is the plan. `code` 401/403 is
  `KEY_REJECTED`; `success: false` is `CONTRACT_CHANGED`.
- **`minimax-api`.** Per `model_remains[]` entry (bucket = `model_name`):
  the interval window lasts `end_time − start_time` (epoch ms; 300 minutes
  → `FIVE_HOURS`, anything else `OTHER`), the weekly one ends at
  `weekly_end_time`. MiniMax confirmed that `current_*_usage_count` holds
  the **remaining** count, so used = `(total − count) / total`; an explicit
  `current_*_remaining_percent` wins when present. A window with a zero total
  and no percentage is skipped. `base_resp.status_code` 1004 or 2049 is
  `KEY_REJECTED`, any other non-zero is `CONTRACT_CHANGED`; no window at all
  is `UNREADABLE`.
- **`commandcode-api`.** `/alpha/billing/credits` is required:
  remaining = `monthlyCredits + purchasedCredits + freeCredits` (read as USD,
  to verify), `fiveHour` and `weekly` windows as `used / cap`, reset at
  `resetAt` (epoch ms). `/alpha/usage/summary` (`totalCost`, spent in the
  billing period) and `/alpha/billing/subscriptions` (`planId`,
  `currentPeriodEnd`) are optional: with both, a `BILLING_CYCLE` meter shows
  `spent / (spent + remaining)` until the period end; without them the
  reading is `PARTIAL`.

### 3.4 OpenCode Zen and declared balances

OpenCode Zen has no balance API for a key; OpenCode records the cost of
every message itself. Ataecina derives the balance from the owner's
declarations and those records:

- `ataecina balance declare <subscription> --top-up <amount>` adds money;
  `--balance <amount>` states the whole current balance and replaces the
  declarations before it. `--currency` defaults to `USD` (Zen bills in
  dollars). Only subscriptions whose source takes declarations
  (`opencode-zen`) accept them.
- Every declaration stores the money spent to date at that moment (the
  *spend baseline*), read from `opencode stats` **lifetime** statistics
  (`--all`), which only ever grow; no day boundary or time zone matters.
  `--limit 1000` keeps every model row; `--standalone` runs a private
  server so no OpenCode background service is left running (to verify).
- At each collection: `remaining = latest declared balance (or 0) + top-ups
  after it − (spent to date − baseline of that reference)` and
  `spent = spent to date − baseline` (never below zero, in case OpenCode's
  records were cleared). `remaining` may go negative when a top-up was not
  declared. Fidelity `DERIVED`. Without any declaration the reading is
  `PARTIAL`, shows only the spending to date and hints at the command.
- Spending sums the `cost` of every entry `{ "model": { "providerID":
  "opencode", … }, "cost": <number> }` wherever it sits in the JSON
  output; other providers are ignored. An output with no model entry and no
  `models` key, a non-numeric or negative cost, or more than 1 MiB is
  `CONTRACT_CHANGED`; `opencode` missing from `PATH` is `UNREADABLE`; a
  failing or slow run (15 s) is `UNAVAILABLE`.
- Declarations live in `$XDG_STATE_HOME/ataecina/balances.json` (`0600`):
  `{ "version": 1, "subscriptions": { "<id>": [ { "kind": "TOP_UP" |
  "BALANCE", "amount": <money>, "declared_at": <RFC 3339>,
  "spent_baseline": <money> } ] } }`, at most 100 top-ups after the last
  declared balance.

Providers without a readable allowance (table above) are shown only through
`heimdall`, or as `UNSUPPORTED` with a hint.

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
  reborn — 0 % used, `estimated: true`, next reset unknown (`resets_at`
  null) — until a new reading arrives. The snapshot keeps the reported
  figures; rebirth is applied only when a display reads it.
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

HTTP answers map as: 2xx read; 401 and 403 `KEY_REJECTED`; 429
`RATE_LIMITED`, with `Retry-After` (seconds or an HTTP date) kept as
`retry_not_before` so the source is not asked before; 5xx, timeouts, DNS
and connection errors `UNAVAILABLE`; any other status, a redirect, a body
over 64 KiB or an unexpected shape `CONTRACT_CHANGED`. A key that is not
sealed shows `KEY_MISSING` with the `keys set` command to run; a master key
no unlocker can open here shows `LOCKED`.

After a transient failure (`RATE_LIMITED`, `UNAVAILABLE`, `UNREADABLE`) the
last reading with figures is kept as it was, with `error_class` and `hint`
recording why it was not refreshed; without such a reading the outcome is
`UNAVAILABLE`. `KEY_REJECTED` shows `KEY_MISSING` and `CONTRACT_CHANGED`
shows `BROKEN`, both without figures. A failure an adapter did not classify,
an adapter that panics, or a source that answers for another subscription
shows `BROKEN` for that subscription only. A source that does not answer
before the collector's deadline is `UNAVAILABLE`. A source this build does
not implement yet shows `UNSUPPORTED`.

## 5. Use cases

| Use case (file) | Context | Actor | Behaviour |
|---|---|---|---|
| `collect_readings` | allowances | collector (timer or `ataecina collect`) | For each enabled subscription, ask its source through the `Source` port (keys via the `KeyRing` port), classify failures, merge with the previous snapshot (keep last good reading on transient errors), write the snapshot atomically. One attempt per source; sources run concurrently with a global deadline (20 s); a source that asked to wait (`retry_not_before`) is not asked. Logs one line per subscription. |
| `report_snapshot` | allowances | display | Read the snapshot, apply freshness and rebirth rules at the current time, return the view model. Never touches keys or the network. |
| `record_claude_limits` | allowances | Claude Code status line hook | Parse the status line JSON from stdin, keep only `rate_limits`, merge them with the kept signal so replays never rewind it, write the Claude signal file only when something is newer (§3.1); print nothing, or a summary line with `--print`. Must finish in a few milliseconds and never fail the status line. |
| `declare_balance` | allowances | owner (`balance declare`) | Read the spending to date through the `RecordedSpend` port, record a top-up or the current balance of a balance-only subscription with that spend baseline, refuse mixed currencies (§3.4). |
| `seal_key` | keys | owner (`keys set`) | Read the key from stdin, seal it under the master key with the provider id as associated data, write `keys.json` atomically. |
| `forget_key` | keys | owner | Remove a sealed key. |
| `list_keys` | keys | owner | List providers that have a sealed key, with the date set; never the key. |
| `initialize_keys` | keys | owner (`init`) | Create the master key and hand it to its first unlocker; refuse when `keys.json` exists. |
| `list_unlockers` | keys | owner, `doctor` | List the unlockers that hold the master key, in the order they are tried. |
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
  exist and whether file permissions are correct. It only reads: no
  network, no unsealing, no prompt. One line per check, `✓` fine, `!` worth
  a look (no keyring on SSH, a source not available yet, an official CLI
  missing from `PATH`, no snapshot yet), `✗` broken (loose or invalid
  configuration, unreadable `keys.json`, a subscription whose key is not
  sealed, no runtime directory); it exits 1 when anything is broken.
- The `passphrase` record in `keys.json` is
  `{ "kdf": "pbkdf2-sha256", "iterations": 600000, "salt": <base64>,
  "nonce": <base64>, "sealed": <base64> }`; the master key is sealed with
  AES-256-GCM under the derived key with `ataecina:v1:master-key:passphrase`
  as associated data. A passphrase has at least 8 characters; on a terminal
  it is asked twice when set. Without a terminal (a systemd service, a pipe)
  the passphrase unlocker is unavailable rather than failing the command,
  unless `--passphrase-fd <n>` gives a file descriptor to read one
  passphrase per line from (scripts and tests; never argv or environment).
- The `secret-service` record in `keys.json` is only
  `{ "attributes": { "application": "ataecina", "purpose": "master-key" } }`;
  the master key itself is an item (label "Ataecina master key", content
  type `application/octet-stream`) created in the collection behind the
  `default` alias, replacing an older one. The session uses the `plain`
  algorithm: the secret crosses the per-user session bus, which only the
  owner's processes reach. The bus is `DBUS_SESSION_BUS_ADDRESS`, or the
  systemd user bus socket `$XDG_RUNTIME_DIR/bus`.
- A locked keyring shows its own unlock prompt only for commands a person
  runs (`init`, `keys set`, `unlockers add`); the collector never prompts
  and treats a locked keyring as an unavailable unlocker. A dismissed
  prompt is an error.
- Unlockers are tried in the order `secret-service`, `systemd-creds`,
  `passphrase`, among those the process can use; the master key is opened
  at most once per process, however many keys a collection unseals.
- `ataecina init` creates `~/.config/ataecina` (`0700`), a minimal
  `config.json` when there is none, and `keys.json` held by
  `--unlocker` (default: `secret-service` when the session's keyring is
  reachable, else `passphrase`). Run twice, it changes nothing.
- `ataecina keys set <name>` reads the key from the terminal without echo,
  or from a pipe: 1 to 4096 printable ASCII characters without spaces, one
  trailing newline removed. It prints only `sealed key <name>`. `keys list`
  prints names and dates; `keys forget` needs no unlocking.
- `ataecina unlockers add <kind>` opens the master key with an existing
  unlocker and wraps it for the new one; `remove` refuses to drop the last
  one and asks the removed unlocker to forget its copy when it can.

## 7. Alerts

- Default thresholds: 90 and 95 % of any meter (`alerts.thresholds`);
  a subscription may set its own (`subscriptions[].thresholds`).
- After every collection the collector hands the meters with figures, as
  they stand now (a reborn window is not watched), to
  `raise_threshold_alerts`. Each crossed threshold is remembered once per
  window instance — subscription, bucket, window and `resets_at`, where two
  resets less than a minute apart are the same window; when several
  thresholds are crossed at once only the highest is announced. The memory
  of a window no longer watched (reborn, or gone) is forgotten, so the next
  window announces again.
- Delivered through `org.freedesktop.Notifications` (Plasma, GNOME, XFCE…)
  with app name "Ataecina", icon `ataecina`, summary
  "`<subscription> · <window> at <n> %`" and the time to rebirth; urgency
  normal, critical from 95 %. Without a session bus (SSH, headless) alerts
  are kept and logged, not shown; a failed notification is not repeated.
  The Plasma widget also highlights the meter.
- State in `$XDG_STATE_HOME/ataecina/alerts.json` (`0600`): `{ "version": 1,
  "raised": [ { "subscription", "bucket", "window", "resets_at",
  "threshold", "raised_at" } ] }`.

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

Contract details:

- `meters` and `subscriptions` are always lists (possibly empty), never
  null. Absent optional values are `null`.
- `source` is the source id in `UPPER_SNAKE_CASE`; `provider` and `id` are
  lowercase identifiers; `plan` is `UPPER_SNAKE_CASE` or `null`.
- Additive fields beyond the example: on a meter, `bucket` (the provider's
  limit name when a subscription has several, e.g. Codex `codex` and
  `premium`; `null` for the main one) and `window_minutes` (`300`, `1440`,
  `10080`, the length of an `OTHER` window, or `null` for `MONTHLY` and
  `BILLING_CYCLE`); on a subscription, `credits`
  (`{ "available": <decimal>, "unlimited": <bool> }`, provider credits that
  are not money), `banked_resets` (`{ "available": <int> }`, information
  only), `error_class` (why the latest attempt failed, §4.3) and `hint` (a
  short sentence for the owner); in `meta`, `snapshot_taken_at` (`null`
  before the first collection) and `worst_used_percent` (`null` without
  meters).
- `used_percent` may exceed 100 when a provider allows overage.
- Before the first collection the document has an empty `subscriptions`
  list; this is not an error.

Errors are RFC 9457 problem objects on stderr with exit code 1 (`2` for
usage errors): `{ "type": "urn:ataecina:problem:<slug>", "title": …,
"detail": … }`. Slugs: `environment` (HOME or XDG_RUNTIME_DIR missing) and
`snapshot-unreadable` (corrupt file, loose permissions).

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

`config.json` example (`etc/ataecina.example.json`):

```json
{
  "version": 1,
  "interval_minutes": 5,
  "alerts": { "thresholds": [90, 95] },
  "network": { "use_environment_proxy": false },
  "subscriptions": [
    { "id": "claude", "source": "claude-statusline", "plan": "PRO" },
    { "id": "codex", "source": "codex-logs", "app_server": false, "app_server_after_minutes": 60 },
    { "id": "zai", "source": "zai-api", "key": "zai" },
    { "id": "openrouter", "source": "openrouter-api", "key": "openrouter" },
    { "id": "zen", "source": "opencode-zen" },
    { "id": "gemini", "source": "heimdall", "provider": "google" }
  ]
}
```

Validation is strict; any violation stops the command before it does
anything:

| Key | Rule |
|---|---|
| `version` | required, `1` |
| `interval_minutes` | 1–1440, default 5 (freshness and the timer use it) |
| `alerts.thresholds` | 1–5 integers rising strictly within 1–100, default `[90, 95]` |
| `subscriptions[].thresholds` | optional, same rules; replaces `alerts.thresholds` for that subscription |
| `network.use_environment_proxy` | default `false` (ADR 0006) |
| `subscriptions[].id` | required, unique, `[a-z0-9][a-z0-9-]{0,31}` |
| `subscriptions[].source` | required, a source id of §3 |
| `subscriptions[].plan` | optional, any text, normalised to `UPPER_SNAKE_CASE`; a plan reported by the source wins |
| `subscriptions[].key` | only for sources that need a key; the sealed key's name, default the subscription id |
| `subscriptions[].app_server`, `app_server_after_minutes` | only for `codex-logs`; default `false` and 60 (≥ 1) |
| `subscriptions[].provider` | only for `heimdall`, where it is required |

Unknown keys, and options given to a source that does not take them, are
errors. The snapshot file is internal (`version`, `taken_at`,
`interval_seconds`, `readings` with the fields of §8.2 plus
`retry_not_before`); only `status --json` is a contract.

## 10. Constraints

- Display path: no network, no decryption, < 50 ms.
- Collector: one-shot, bounded by a global deadline (default 20 s), memory
  well under 30 MB.
- Single static binary (`CGO_ENABLED=0`); runtime dependencies: none beyond
  the session D-Bus.
- Linux only. Plasma 6 is the reference desktop; others through the tray.
