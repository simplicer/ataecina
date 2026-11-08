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
| **Offering** | A model a provider offers, with its list price and release day; **free** when priced at 0, **new** when released in the last 30 days. |
| **Model catalogue** | The offerings of the providers that describe the configured subscriptions, refreshed once a day from models.dev (§11). |

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
for subscriptions the owner enabled; the one other host is the public model
catalogue `https://models.dev`, asked at most once a day without any key
(§11), and, only when the owner runs `ataecina update` with its key, the
Artificial Analysis Intelligence Index at `https://artificialanalysis.ai`
(§11.1) and the published offers at `https://codeberg.org` (§12). **Undocumented** sources must be
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
| `alibaba-cli` | Alibaba Model Studio Token Plan | official CLI | `bl usage token-plan --output json` (executed with a fixed argv, no shell) (§3.3) | `bl`'s own console login (`bl auth login --console`) | every `per<N><Hour|Day|Week|Month>Percentage` (used fraction 0–1) and `…ResetTime` (epoch ms), `hasQuotaData` | yes (Model Studio docs; field names in bl's source) | REPORTED | docs and bl source 2026-11-08; `bl` not installed, to verify |
| `alibaba-coding-cli` | Alibaba Model Studio Coding Plan | official CLI | `bl usage coding-plan --output json` (fixed argv, no shell) (§3.3) | `bl`'s own console login | `instanceType`; `per5Hour`, `perWeek`, `perBillMonth`: `usedQuota`, `totalQuota`, `percentage` (used fraction), `resetTime` (epoch ms) | yes (Model Studio docs; field names in bl's source) | REPORTED | docs and bl source 2026-11-08; to verify |
| `commandcode-api` | Command Code | provider API | `https://api.commandcode.ai` `/alpha/billing/credits`, `/alpha/usage/summary`, `/alpha/billing/subscriptions` (§3.5) | API key (`Bearer`) | `credits.{monthly,purchased,free}Credits`, `windowLimits.{fiveHour,weekly}.{used,cap,resetAt}`, `totalCost`, `data.{planId,currentPeriodEnd}` | no (used by the official CLI) | REPORTED | third-party 2026-11-06; live to verify |
| `openrouter-api` | OpenRouter | provider API | `https://openrouter.ai/api/v1/key`, `/api/v1/credits` (§3.5) | API key (`Bearer`; `/credits` may need a management key) | `data.{usage,limit,limit_remaining,free_model_daily_requests}`, `data.{total_credits,total_usage}` | yes | REPORTED | docs 2026-11-06 |
| `vercel-api` | Vercel AI Gateway | provider API | `https://ai-gateway.vercel.sh/v1/credits` (§3.5) | API key (`Bearer`) | `balance`, `total_used` (USD decimal strings) | yes | REPORTED | docs 2026-11-06 |
| `opencode-zen` | OpenCode Zen | official CLI + declaration | `opencode stats --all --models --limit 1000 --json --standalone` (fixed argv, no shell), entries with `model.providerID = "opencode"` | none | per-model `cost` to date, against the declared balance (§3.4) | yes (CLI help) | DERIVED | local 2026-11-06 (opencode 2.2.5 `--help`); JSON shape and `--standalone` to verify |
| `opencode-console` | OpenCode Go, OpenCode Console budgets | official CLI | `opencode api experimental.usage.limits --standalone` (fixed argv, no shell) (§3.4) | OpenCode's own sign-in to an OpenCode Console workspace | `data` (null when signed out); `data.go.{rolling,weekly,monthly}.{percent,limited,resetsAt}`; `data.budgets[].{limit,spent,resetsAt}` | experimental operation of OpenCode's local server, schema in the opencode 2.2.5 binary | REPORTED | local 2026-11-08 (signed-out answer); Go and budgets to verify |
| `moonshot-api`, `moonshot-cn-api` | Moonshot AI (Kimi) open platform | provider API | `https://api.moonshot.ai/v1/users/me/balance` (USD) or `https://api.moonshot.cn/…` (CNY) (§3.5) | platform API key (`Bearer`), valid only on its own platform | `code`, `status`, `data.{available_balance, voucher_balance, cash_balance}` | yes | REPORTED | docs 2026-11-08; live to verify |
| `kimi-code-api` | Kimi Code membership (Kimi Plus, Pro, Max, Ultra) | provider API | `https://api.kimi.com/coding/v1/usages` (§3.5) | an API key created in the Kimi Code Console (`Bearer`); never the OAuth token of `kimi login` | `usage.{limit, remaining, resetTime}` (weekly), `limits[].{window.{duration, timeUnit}, detail.{limit, remaining | used, resetTime}}`, `user.membership.level` | no (called by the official kimi-cli `/usage`) | REPORTED | official CLI source 2026-11-08; live to verify |
| `xai-api` | xAI API (Grok) prepaid credit | provider API | `https://management-api.x.ai/v1/billing/teams/<team>/prepaid/balance` (§3.5) | a management key (`Bearer`, console Settings → Management Keys) and the team id | `total.val` (US cents, string) | yes | REPORTED | docs 2026-11-08; sign convention live to verify |
| `cursor-admin-api` | Cursor Teams / Enterprise (one member) | provider API | `POST https://api.cursor.com/teams/spend` (§3.5) | the team's Admin API key (Basic, key as user name) and the member's e-mail | `teamMemberSpend[].{email, spendCents, overallSpendCents, effectivePerUserLimitDollars, hardLimitOverrideDollars, monthlyLimitDollars}`, `subscriptionCycleStart` | yes | REPORTED | docs 2026-11-08; live to verify |
| `ollama-api` | Ollama Cloud | provider API | `https://ollama.com/api/balance` (§3.3) | Ollama API key (`Authorization: Bearer`) | `included.{balance_usd, allowance_usd, period.until}`; legacy plans `included.{session, weekly}.{remaining_percent, resets_at}`; `purchased.balance_usd` | yes (docs.ollama.com/api/balance) | REPORTED | docs and live 2026-11-08 (credit plan) |
| `copilot-gh` | GitHub Copilot | official CLI | `gh api` (fixed argv, no shell) → `https://api.github.com/users/<login>/settings/billing/premium_request/usage?year=<Y>&month=<M>`, or the AI-credit usage report for a user (same schema, path to verify) (§3.3) | `gh`'s own login, which needs the "Plan" read permission (scope to verify) | `usageItems[]` with `product` Copilot: `grossQuantity`, `discountQuantity`, `netQuantity`, `netAmount`; monthly allowance declared in `config.json` | yes (GitHub REST API `2026-03-10`) | DERIVED | docs 2026-11-06; live to verify |
| `orcarouter-api` | Orca Router | provider API | `https://api.orcarouter.ai/v1/dashboard/billing/subscription`, `/v1/dashboard/billing/usage` | API key (`sk-orca-…`) | remaining allowance and expiry; cumulative usage (`TotalUsage`; JSON names and units to verify) | yes (OrcaRouter "Billing & usage") | REPORTED | docs 2026-11-06; live to verify |
| `cloudflare-api` | Cloudflare Workers AI | provider API | `https://api.cloudflare.com/client/v4/graphql` (GraphQL Analytics, one fixed query) (§3.3) | API token with *Account Analytics: Read* only, plus the account id | `aiInferenceAdaptiveGroups` → `sum.totalNeurons` for the current UTC day, against the published 10 000 neurons/day allocation | yes (GraphQL Analytics API; dataset fields to verify) | REPORTED | docs and third-party 2026-11-06; live to verify |
| `heimdall` | any plan Heimdall serves | local service | `http://127.0.0.1:<port>/v1/plans`, Heimdall's admin API on loopback, port 4001 by default (§3.6, ADR 0013) | Heimdall admin token (`Bearer`) | `data[]`: `id`, `provider`, `available`, `unavailable_reason`, `next_reset_at`, `window_used_ratio` | yes (Heimdall OpenAPI `admin.yaml` 0.23.1) | OBSERVED | local 2026-11-06 (contract); Heimdall 0.23.1 does not serve `available`, `unavailable_reason` or `window_used_ratio` yet |

### Providers without a readable allowance

| Provider | What exists (2026-11-06) | Decision |
|---|---|---|
| Google AI Studio (Gemini API keys) | Limits are per Google Cloud project, not per key; requests per day reset at midnight Pacific time (docs). Usage is shown only in the AI Studio console; reading Cloud quota metrics needs a Google OAuth or service-account credential. | `heimdall` (`OBSERVED`: `429 RESOURCE_EXHAUSTED`, daily rebirth at midnight Pacific), else `UNSUPPORTED` |
| Gemini CLI with a Google account | Quota is shown only inside a session (`/stats`); the CLI has no non-interactive usage command (`gemini --help`, 0.1.9) and its quota call needs the Google OAuth token (ADR 0003). | `UNSUPPORTED`. Candidate, to verify: a `DERIVED` request count from the CLI's opt-in local telemetry file (`telemetry.target: local`, prompts not logged), read for event names only |
| Qwen Code (Qwen OAuth) | The free Qwen OAuth tier was discontinued on 2026-04-15 (`qwen auth` reports "removed" in 0.21.8). Qwen Code on a Model Studio plan is covered by `alibaba-cli`. | `UNSUPPORTED` |
| NVIDIA (build.nvidia.com) | Trial credits and a per-minute rate limit, visible only in the console; no usage API. | `heimdall` (`OBSERVED`), else `UNSUPPORTED` |
| Groq | Documented `x-ratelimit-limit/remaining/reset-requests` (per day) and `-tokens` (per minute) headers on every inference response; no endpoint answers without spending a request. | `heimdall` (`OBSERVED` from the headers of traffic it already serves) |
| LLM7 | Free-token quotas (tokens per day, requests per second/minute/hour) that may change without notice; only a models catalogue is documented. | `heimdall` (`OBSERVED`), else `UNSUPPORTED` |
| Cursor individual plans (Free, Plus, Premium, Super, Ultra) | A weekly allowance shared by Cursor and Grok, relaunched on 2026-10-12, in multiples of Plus with no published figures. Usage is shown only in the dashboard and the IDE, through the session token (`cursor.com/api/usage-summary`); `api.cursor.com/v0/me` with a user API key gives no usage. | `UNSUPPORTED` (teams: `cursor-admin-api`) |
| Windsurf / Devin Desktop (Free, Pro, Max, Teams) | Daily and weekly quotas since 2026-03-19, reset by calendar date; readable only with the browser or IDE session token. The Enterprise service-key API (`GetTeamCreditBalance`) still speaks of prompt credits, whose meaning under quotas is unknown. | `UNSUPPORTED` until an Enterprise owner verifies the service-key answer |
| SuperGrok / X subscription | A weekly pool per seat, shown only in the account. No API, no CLI. | `UNSUPPORTED` (the API credit: `xai-api`) |
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

Milestone 9 implements these adapters against synthetic answers built from
the shapes below; each must be verified with a live, manual call before a
release, like the undocumented sources of §3.5. Official CLIs run through
one runner (`internal/platform/officialcli`): the binary found on `PATH`
(absolute entries only), a fixed argv, no shell, stderr discarded, stdout
capped, a timeout. The collector unit's `PATH` can be extended with a
drop-in when `gh` or `bl` live elsewhere.

**`ollama-api` (Ollama Cloud).** The `ollama` CLI (0.31.1) has no usage
command. Ollama documents `GET https://ollama.com/api/balance` with the
owner's API key as `Authorization: Bearer` (no query; 10 requests per minute
per user, shared by all keys; errors are `{ "error": … }` with `400`, `401`,
`403`, `429` with `Retry-After`, or `503`). Read live on 2026-11-08:

- **credit plans** (every plan started since 2026-08-31: Pro $60 and Max
  $300 of included usage a month, renewed on the subscription day; it does
  not roll over and is spent before purchased credit) answer `"included": {
  "balance_usd", "allowance_usd", "period": { "from", "until" } }` (RFC 3339,
  `until` exclusive: the renewal). Ataecina shows a `BILLING_CYCLE` meter
  `(allowance_usd − balance_usd) ÷ allowance_usd`, reborn at `until`, and the
  balance: spent = `allowance_usd − balance_usd`, left = `balance_usd` plus
  any purchased credit;
- **legacy plans** (Pro and Max subscribed before then) answer `"included":
  { "session", "weekly" }`, each `{ "remaining_percent": 0–100, "resets_at" }`
  → `FIVE_HOURS` and `WEEKLY` meters, used = 100 − `remaining_percent`;
- `"purchased": { "balance_usd" }` is purchased credit not yet expired
  (pay-as-you-go accounts have only this), added to the money left;
- an answer with none of these figures, a negative amount, a percentage
  outside 0–100 or an instant that is not RFC 3339 is `CONTRACT_CHANGED`.

`GET /api/usage` (documented too) is a usage history, not a balance:
`range` `24h`, `7d` or `30d`, daily or hourly buckets of `usage_usd`, tokens
and requests; Ataecina does not read it. The same key can spend credits, so
it is sealed like any other key.

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
the monthly allowance are declared in `config.json` (`login`,
`monthly_allowance`). Used percentage = `Σ grossQuantity` of the items whose
`product` is Copilot ÷ declared allowance (`DERIVED`); the `MONTHLY` window
is reborn on the 1st at 00:00 UTC (documented); `Σ netAmount` (USD) is the
overage spent. A month without Copilot items reads 0 % with a hint. `gh`
runs with `GH_NO_UPDATE_NOTIFIER=1`, `GH_PROMPT_DISABLED=1` and `NO_COLOR=1`,
a 15 s timeout and 1 MiB of output at most. The user report only covers
plans the owner pays personally; the AI-credit report path is still to
verify, so AI-credit plans may read as an empty month.

**`alibaba-cli` and `alibaba-coding-cli` (Alibaba Model Studio).** Model
Studio has no quota API for the plan's API key (`sk-sp-…` keys only serve
inference on `coding-intl.dashscope.aliyuncs.com`); the official Bailian
CLI `bl` (Apache-2.0, checked against bl 2.8.0) reads the quota through
the Model Studio console with its own console login (`bl auth login
--console`, once, in a browser). Ataecina runs `bl` with a fixed argv, no
shell, `NO_COLOR=1`, a 15 s timeout and 64 KiB of output at most, and never
calls the console gateway itself nor reads bl's configuration.

- **Token Plan** (`bl usage token-plan --output json`): one flat object
  whose windows depend on the plan and the bl version; Ataecina reads every
  `per<N><Hour|Day|Week|Month>Percentage` (a used fraction, 0–1) with its
  `…ResetTime` (epoch ms), as bl itself does: hours, days and weeks by
  length (5 hours → `FIVE_HOURS`, 1 day → `DAILY`, 1 week → `WEEKLY`,
  others `OTHER`), `per1Month…` → `MONTHLY`; longer calendar spans are
  skipped. `hasQuotaData: false` is `UNAVAILABLE`; no window at all is
  `CONTRACT_CHANGED`.
- **Coding Plan** (`bl usage coding-plan --output json`): `instanceType`
  (the plan: `lite`, `pro`) and `per5Hour`, `perWeek`, `perBillMonth`, each
  `{ "usedQuota", "totalQuota", "percentage", "resetTime" }` in requests;
  `percentage` (else `usedQuota ÷ totalQuota`) gives `FIVE_HOURS`,
  `WEEKLY` and `BILLING_CYCLE` meters. Published limits (2026-11-04): Pro,
  $50 a month, 6 000 requests per 5 hours (rolling), 45 000 per week
  (Monday 00:00 UTC+8) and 90 000 per month (renewal day, 00:00 UTC+8); Lite
  closed to new subscribers. An empty answer `{}` (no valid subscription)
  is `UNAVAILABLE`.

Team accounts that need `--region`/`--site` flags are not handled yet.

**`orcarouter-api` (Orca Router).** OrcaRouter documents two summary
endpoints authenticated with the API key: `/v1/dashboard/billing/subscription`
(remaining allowance and expiry) and `/v1/dashboard/billing/usage`
(cumulative usage). Rate limits are per workspace and only `Retry-After` is
returned. Key limits may renew daily, weekly or monthly (UTC); there is no
documented call to read a key's own limit. Both summaries follow the
OpenAI-compatible billing shape, which Ataecina reads (to verify live):
`hard_limit_usd` (the key's total allowance in USD; 1 000 000 or more means
no limit) and `access_until` (Unix seconds, 0 when it does not expire) from
the first, `total_usage` (US cents) from the second. Balance: spent =
`total_usage` ÷ 100; left = `hard_limit_usd` − spent (never below 0); the
expiry is the hint. Both calls are required.

**`cloudflare-api` (Cloudflare Workers AI).** Workers AI grants 10 000
neurons per day on every plan. The GraphQL Analytics API
(`POST https://api.cloudflare.com/client/v4/graphql`) exposes the
`aiInferenceAdaptiveGroups` dataset; Ataecina sends one fixed query
filtered by `accountTag` and today's UTC date and reads `sum.totalNeurons`.
The token needs only *Account Analytics: Read* (it cannot run inference).
The `DAILY` window is reborn at 00:00 UTC (to verify); analytics may lag a
few minutes behind the dashboard. The query text is fixed in the binary;
only the variables change:

```graphql
query AtaecinaNeurons($accountTag: string!, $date: Date!) {
  viewer { accounts(filter: {accountTag: $accountTag}) {
    aiInferenceAdaptiveGroups(limit: 10000, filter: {date_geq: $date}) { sum { totalNeurons } }
  } }
}
```

It is sent as `POST /client/v4/graphql` with the token as `Bearer`. The
neurons of every returned group are summed; used = neurons ÷ 10 000 × 100.
A GraphQL error saying "not authorized", or an answer without the account,
is `KEY_REJECTED`; any other GraphQL error is `CONTRACT_CHANGED`.

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
- **`moonshot-api`, `moonshot-cn-api`.** The documented balance of the
  Moonshot open platform: `available_balance` (= cash + vouchers; at zero or
  below, inference stops) is the money left, in USD on `api.moonshot.ai`
  and in CNY on `api.moonshot.cn`; `cash_balance` below zero (money owed)
  adds a hint. `code` other than 0 or `status` false is `CONTRACT_CHANGED`.
  It is prepaid: there is no window and no reset.
- **`kimi-code-api`.** The Kimi Code membership's windows, as the official
  kimi-cli reads them for `/usage`, with a key from the Kimi Code Console
  (`Bearer`; the OAuth token `kimi login` stores is never read). Each
  `limits[]` entry is a window of `duration` × `timeUnit`
  (`TIME_UNIT_MINUTE`, `_HOUR`, `_DAY`; 300 minutes → `FIVE_HOURS`, 7 days
  → `WEEKLY`, else `OTHER`); the top-level `usage` is the weekly window
  (legacy plans; new plans have only the five-hour one) and is skipped when
  a `limits[]` entry already gave it. Numbers may be strings; used =
  `used` or `limit − remaining`, as a share of `limit`; `resetTime` is RFC
  3339. `user.membership.level` minus `LEVEL_` is the plan. The new plans'
  shared monthly credit pool is not in this answer.
- **`xai-api`.** The documented Management API with a management key
  (`Bearer`; created in the console with *Management Keys* read access)
  and the team id declared in `config.json` (`team`; it is also the
  `team_id` that `GET https://api.x.ai/v1/api-key` gives for any of the
  team's API keys). `total.val` is the prepaid balance in US cents as a
  string; purchases are recorded as negative changes, so the money left is
  the total's magnitude (sign convention to verify live). No window.
- **`cursor-admin-api`.** Cursor's documented Admin API for Teams and
  Enterprise plans, with the team's Admin API key as the Basic user name
  and an empty password. One `POST /teams/spend` with `{ "searchTerm":
  <email>, "pageSize": 100 }` (the e-mail of the member, declared in
  `config.json` as `email`; it is sent only to Cursor) gives the member's
  row: `spendCents` (on-demand, billed beyond the plan) and
  `overallSpendCents` (with the included usage), in cents, and the member's
  on-demand limit in dollars (`effectivePerUserLimitDollars`, else
  `hardLimitOverrideDollars`, else `monthlyLimitDollars`). With a limit: a
  `BILLING_CYCLE` meter `on_demand` = on-demand ÷ limit, reborn at the
  next monthly anniversary of `subscriptionCycleStart` (epoch ms), money
  left = limit − on-demand, spent = overall. Without a limit: `PARTIAL`,
  the spending only. A member not in the answer is `UNAVAILABLE`. Cursor
  publishes no "remaining" of the included weekly allowance.
- **`commandcode-api`.** `/alpha/billing/credits` is required:
  remaining = `monthlyCredits + purchasedCredits + freeCredits` (read as USD,
  to verify), `fiveHour` and `weekly` windows as `used / cap`, reset at
  `resetAt` (epoch ms). `/alpha/usage/summary` (`totalCost`, spent in the
  billing period) and `/alpha/billing/subscriptions` (`planId`,
  `currentPeriodEnd`) are optional: with both, a `BILLING_CYCLE` meter shows
  `spent / (spent + remaining)` until the period end; without them the
  reading is `PARTIAL`.

### 3.6 Heimdall (local, milestone 9)

Heimdall, the owner's model relay, publishes the plans it serves on its
admin API: `GET /v1/plans` returns `{ "data": [ plan… ], "meta", "links" }`
where a plan has `id`, `provider`, `available`, `unavailable_reason`
(`peak_window`, `quota_exhausted`, `insufficient_balance`,
`unauthorized_account` or null), `next_reset_at` (RFC 3339 or null) and
`window_used_ratio` (0–1 or null). A `heimdall` subscription names the
`provider` whose plans it follows (`google` for Gemini API keys, `nvidia`,
`groq`, `llm7`, `kilo`…):

- one plan → one meter in the main bucket; several → one bucket per plan id;
- the meter is the plan's current window (`BILLING_CYCLE`, since Heimdall
  gives the next reset but not the window's length), used =
  `window_used_ratio` × 100, reborn at `next_reset_at`;
- a plan unavailable with `quota_exhausted` and no ratio is 100 %; every
  unavailable plan adds its reason to the hint (unknown reasons read as
  "unavailable", never echoed);
- no plan of the provider, or no plan with usage, is `UNAVAILABLE` with the
  known next resets in the hint: nothing is estimated.

Heimdall 0.23.1 implements `GET /v1/plans` with `next_reset_at` but not yet
the availability and usage fields of its own contract; until it does, these
subscriptions read `UNAVAILABLE`. The adapter talks to `127.0.0.1` only, over
plain HTTP, on the configured `port` (ADR 0013).

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
- **OpenCode Go and Console budgets (`opencode-console`, verified locally
  2026-11-08 with opencode 2.2.5 signed out).** OpenCode's local server
  offers the experimental operation `experimental.usage.limits` (`GET
  /api/experimental/usage/limits`). Ataecina runs it through the official CLI
  with a fixed argv, `opencode api experimental.usage.limits --standalone`
  (no shell, `NO_COLOR=1`, 15 s, 64 KiB), so OpenCode uses its own sign-in
  and Ataecina never sees a credential. The answer is `{ "location": …,
  "data": null }` when OpenCode is not signed in to an OpenCode Console
  workspace (`UNAVAILABLE`, with the hint to run `opencode auth login`),
  otherwise `data` is `{ "workspace": { "id", "name"?, "usageURL" }, "go"?:
  { "rolling", "weekly", "monthly" }, "budgets": [ … ] }` (schema
  `UsageLimits.Info` in the opencode 2.2.5 binary):
  - each Go window is `{ "percent": 0–100 used, "limited": bool, "resetsAt":
    epoch ms }` → meters `FIVE_HOURS` (rolling, length to verify), `WEEKLY`
    and `MONTHLY` in the main bucket; `limited: true` reads as 100 %; the
    plan reads `GO` unless the owner declared one;
  - each budget is `{ "id", "limit", "spent" (USD), "scope", "per",
    "coverage", "models", "vendors", "picked", "shared", "resetsAt" }` → a
    `BILLING_CYCLE` meter `spent ÷ limit` in bucket `budget` (`budget_1`,
    `budget_2`… when there are several) reborn at `resetsAt`; a single budget
    is also the balance (left = `limit − spent`, never below zero); a budget
    with a zero limit is skipped;
  - no Go plan and no budget is `UNAVAILABLE`; a missing `data` or
    `budgets`, an incomplete Go window, a percent outside 0–100, a negative
    amount or a reset instant not in epoch milliseconds is
    `CONTRACT_CHANGED`.

  It gives what is left of the Go allowance and of the budgets the owner
  set, not the prepaid Zen balance, which stays declared (above).
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
- `ataecina unlockers add systemd-creds` runs `systemd-creds encrypt --user
  --name=ataecina.master-key - ~/.config/ataecina/master-key.cred` (fixed
  argv, master key on stdin) and writes the drop-in
  `~/.config/systemd/user/ataecina-collect.service.d/master-key.conf` with
  `LoadCredentialEncrypted=ataecina.master-key:<file>`; the owner then runs
  `systemctl --user daemon-reload`. Inside the unit the collector reads
  `$CREDENTIALS_DIRECTORY/ataecina.master-key`; elsewhere the unlocker runs
  `systemd-creds decrypt --user`. Removing it deletes both files. The
  `keys.json` record holds only the credential name and the two paths.
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
  prints names and dates; `keys forget` needs no unlocking. The Plasma
  widget's Keys settings page feeds it from `kdialog --password` through a
  pipe (§8.3, ADR 0014).
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
- **Failing sources:** after the thresholds, `notify_failing_sources`
  watches every subscription's outcome. A source fails when its outcome
  carries no figures (`BROKEN`, `UNAVAILABLE`, `KEY_MISSING`, `LOCKED`,
  `UNSUPPORTED`); a transient failure that keeps the last good reading is
  not one. Each source is announced once when it starts failing, and again
  only if it fails for another reason; a source that recovers, or leaves the
  configuration, is forgotten, so failing again is announced again. The
  notification reads "`<subscription> · <outcome>`" (`workers-ai · key
  missing`) with the source's hint as body, urgency normal, and the log gets
  one `source failing` line. The Plasma widget lists the same sources under
  its warnings (§8.3).
- State in `$XDG_STATE_HOME/ataecina/alerts.json` (`0600`): `{ "version": 1,
  "raised": [ { "subscription", "bucket", "window", "resets_at",
  "threshold", "raised_at" } ], "failing": [ { "subscription", "failure",
  "since" } ] }`; a file without `failing` (written before 0.19.0) still
  loads.

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
  only), `error_class` (why the latest attempt failed, §4.3), `hint` (a
  short sentence for the owner) and `offerings` (§11; `null` when the
  catalogue has nothing for the subscription): `{ "catalogue": <models.dev
  provider id>, "name", "models": <int>, "free": <int>, "free_models":
  [ { "id", "name", "released_on" } ] (at most 50), "new_models": [ … ]
  (released in the last 30 days, newest first, at most 10) }`, where
  `released_on` is a date (`2026-10-30`) or `null`; in `meta`,
  `snapshot_taken_at` (`null` before the first collection),
  `worst_used_percent` (`null` without meters) and
  `offerings_refreshed_at` (`null` before the catalogue was first read).
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

- **Where it lives:** in a panel, or inside the system tray among its
  entries (`X-Plasma-NotificationArea`, category `SystemServices`), where
  Plasma shows it when relevant and in plain view while it asks for
  attention. On Plasma the widget in the tray replaces `ataecina tray`.
- **Figures show what is left** (`100 − used_percent`), as the question is
  "how much is left"; the JSON contract keeps `used_percent`.
- **Colours:** Ataecina's own palette, not the providers' brands. Growth
  rings are teal `#2e8b74` (outer, longest window), green `#4caf50` and light
  green `#9ccc65` (inner); a window turns amber `#e0a13a` from 90 % used and
  coral `#e2604a` from 95 % used (the default thresholds — the contract
  carries no thresholds, so a subscription's own thresholds change its
  alerts only); grey when the reading is not both `FRESH` and `OK` or
  `PARTIAL`. Text and backgrounds follow the Plasma colour scheme, light or
  dark.
- **Compact form (panel or tray icon):** one ring around the Ataecina icon
  showing what is left of the window closest to running out; in a panel,
  optionally that percentage beside it. The widget asks for attention from
  90 % used. Tooltip: that subscription, window, what is left and the time
  to rebirth. Left click opens the popup; middle click refreshes.
- **What gets a row, a warning or nothing:** a subscription whose source
  failed (any outcome but `OK` and `PARTIAL`) is a **warning**; one with
  windows, or with money left (`balance.remaining` above zero), gets a
  **row**; so does a `PARTIAL` reading that knows only what was spent (an
  OpenCode Zen balance not declared yet: "$1.20 spent", with a note to
  declare it in the settings); one that answers with nothing worth showing
  — a free credit used up, or only what was spent of an `OK` reading — is
  **hidden**, unless its source asks for something (`PARTIAL` with a hint
  and no figure), which makes it a warning. So a free provider without a
  plan shows only when it reports free-model limits.
- **Full form (popup):** one rounded card with a row per
  listed subscription:
  - growth rings for what is known only: one per window of the first
    bucket, longest outside, each filled with what is left; for money left
    of a known total (left plus spent), one ring with the currency inside;
    no ring at all when nothing is known; grey and half transparent when the
    reading is not fresh;
  - the name (the provider's, or the subscription id when two share a
    source), the plan in grey, and the money left of its total (`$6.80 of
    $70.00`, or `$6.80 left` when the total is unknown);
  - **what is left of every window** of the first bucket, shortest first
    (`5 h`, `week`, `month` or `cycle`), each as a large percentage (`≈`
    when estimated after a rebirth) over its countdown (`↻ 3 d 17 h`) or
    `reborn`;
  - a small note: how a stale reading gets fresh again (`Updates when you
    use Claude Code`, `Updates when you use Codex`, else `Stale, observed 2
    hours ago`) and banked resets;
  - a click unfolds every bucket and window (label, bar of what is left,
    `% left`, countdown), the balance and credits, fidelity and outcome,
    the hint and the reading's age.

  Below the card, the **warnings** fold under one line (`⚠ 4 warnings`
  and their names); unfolded, each shows its outcome and hint. A footer
  shows when the snapshot was taken (`Checked 11:12`), "Refresh now" and the
  settings; the context menu also offers "Refresh now".

```text
┌───────────────────────────────────────────────────────┐
│ ╭───────────────────────────────────────────────────╮ │
│ │ ◎  Claude  Pro                                   ⌄│ │
│ │    5 h        week                                │ │
│ │    76 %       59 %                                │ │
│ │    ↻ 2 h 10 m ↻ 4 d 12 h                          │ │
│ │ ───────────────────────────────────────────────── │ │
│ │ ◎  Codex  Plus                                   ⌄│ │
│ │    5 h        week                                │ │
│ │    92 %       37 %                                │ │
│ │    ↻ 3 h 0 m  ↻ 2 d 7 h                           │ │
│ │    1 banked reset                                 │ │
│ │ ───────────────────────────────────────────────── │ │
│ │ ◎  Command Code  Pro              $6.80 of $70.00 ⌄│ │
│ │    5 h        week       cycle                    │ │
│ │    100 %      96 %       9 %                      │ │
│ │               ↻ 2 d 0 h  ↻ 10 d 7 h               │ │
│ ╰───────────────────────────────────────────────────╯ │
│ ╭───────────────────────────────────────────────────╮ │
│ │ ⚠ 2 warnings   Ollama Cloud, Workers AI           ⌄│ │
│ ╰───────────────────────────────────────────────────╯ │
│ Checked 11:12                                   ⟳  ⚙ │
└───────────────────────────────────────────────────────┘
```

- It runs `ataecina status --json` on a timer (default 60 s, 10–3600 s)
  through the Plasma 5 support executable engine; "refresh now" runs
  `systemctl --user start ataecina-collect.service` and reads the snapshot
  again four seconds later. It never collects, never reads keys and has no
  network code. A failing command or an unreadable answer is shown as a
  message in the popup and the tooltip.
- Settings page (`contents/config/main.xml`): the command (`ataecina`, or a
  full path when it is not on `plasmashell`'s `PATH`), the refresh interval,
  whether what is left shows in the panel, and which subscriptions to show
  in which order (comma-separated ids; empty shows all in configuration
  order).
- **Subscriptions** settings page: lists the subscriptions of `config.json`
  (`ataecina subscriptions --json`), removes one after a second click
  (`ataecina subscriptions remove <id>`) and adds one: the source is chosen
  from `ataecina sources --json`, an id is suggested (the provider, or
  `minimax-2` when `minimax` is taken), only the source's required options
  are asked, and `ataecina subscriptions add` validates the result like
  `collect` does before writing it. Every argument is single-quoted for the
  shell; the widget never writes the file itself. Two subscriptions of the
  same source differ by id and by key name (default: the id).
- **Keys** settings page (ADR 0014), read from `ataecina status --json`
  (and `ataecina sources --json` for which sources need a key):
  every subscription whose source needs a key, with `works`, `no key`,
  `key rejected` (`error_class` `KEY_REJECTED`, which also reads
  `KEY_MISSING`), `key accepted, answer not understood` (`BROKEN`) or `not
  read yet`, and "Set key…" / "Replace key…": `kdialog --password` piped
  into `ataecina keys set '<id>'`. OpenCode Zen is listed with its balance
  and "Declare balance…": `kdialog --inputbox`, then `ataecina balance
  declare '<id>' --balance "$amount"` with the collector unit's `PATH`
  directories first. After either, the page starts the collector and reads
  the status again four seconds later; errors (such as a passphrase
  unlocker, which needs a terminal) show on the page. Without `kdialog` the
  buttons are disabled and the terminal command is shown.
- Translatable through KDE i18n (domain `plasma_applet_org.simplicer.ataecina`):
  English source strings in `translate/template.pot`; Spanish (the main
  language), Catalan, Basque and Galician in `translate/{es,ca,eu,gl}.po`;
  `make widget` compiles them into `contents/locale/` of the staged package.
  Symbols (`↻`, `·`, `≈`) stay outside the translated strings. Percentages
  are translated (`%1%`: `63%` in English, `63 %` in Spanish, Catalan and
  Galician, `% 63`/`%63` in Basque, where the big figures also put the sign
  first); money and dates follow Qt's locale. Desktop notifications use the
  same five languages, chosen from `LC_ALL`, `LC_MESSAGES`, `LANG` and
  `LANGUAGE` like gettext, English otherwise.
- Checks: `tests/widget` (run by `go test`) keeps `metadata.json` at
  `VERSION`, the settings schema in step with the QML, every string in the
  template and translated, and every field the QML reads inside this
  contract; `make check` runs `qmllint` when it is installed and `msgfmt
  --check` always.

### 8.5 Subscriptions and sources from the CLI

`config.json` can be edited by hand or through the CLI, which keeps its
layout (one subscription per line), validates the whole result exactly as
`collect` loads it, and replaces the file atomically (`0600`):

- `ataecina subscriptions [--json]` lists them; `--json` gives `{ "data": [ {
  "id", "source", "label" (`UPPER_SNAKE_CASE`, as in `status --json`),
  "provider", "plan" (or null), "key" (the sealed key's name, or null),
  "options": { <option>: <value> } } ] }`.
- `ataecina subscriptions add <id> --source <source> [--plan <plan>]
  [--key <name>] [--set <option>=<value>]…` appends one (numbers and
  switches are written as such); it prints the `keys set` command when the
  source needs a key.
- `ataecina subscriptions remove <id>` drops one; its sealed key stays until
  `ataecina keys forget`.
- `ataecina sources [--json]` lists every source: `{ "data": [ { "id",
  "label", "provider" (null when the owner names it), "fidelity",
  "needs_key", "options", "required", "takes_declarations" } ] }`.

These documents are read by the widget's settings pages and are additive
like `status --json`.

### 8.4 Tray (`ataecina tray`)

StatusNotifierItem with the same icon and a menu listing every subscription;
for XFCE, Cinnamon, MATE, LXQt, Budgie, Waybar and GNOME with the AppIndicator
extension.

- **Item** (`org.kde.StatusNotifierItem` at `/StatusNotifierItem`, bus name
  `org.kde.StatusNotifierItem-<pid>-1`): icon `ataecina`; status
  `NeedsAttention` from 90 % (as the widget), `Active` otherwise; tooltip with
  the snapshot's age and one line per subscription; `ItemIsMenu`, so a click
  opens the menu; a middle click refreshes.
- **Menu** (`com.canonical.dbusmenu` version 3 at `/MenuBar`): one line per
  subscription as in the terminal (`claude · PRO · 5h 23% ↻ 2h10m · week 41%
  ↻ 4d12h · fresh 1m ago`), or why there is nothing to show; then the
  snapshot's age, "Refresh now" and "Quit". A refresh that systemd refuses is
  shown as a line until the next one.
- It reads the snapshot every 30 s and 4 s after "Refresh now", which asks
  the user's systemd manager over the session bus to start
  `ataecina-collect.service` (`StartUnit`, mode `replace`); it runs no
  program, never collects and never unseals.
- It registers with `org.kde.StatusNotifierWatcher`, waits for one when the
  panel has none yet and registers again when the panel restarts. It ends on
  "Quit", `SIGINT` or `SIGTERM`.
- Launcher `org.simplicer.ataecina.desktop` (`Exec=<bindir>/ataecina tray`),
  offered on every desktop: on Plasma the widget is the main display, but
  the tray works there too (Plasma's system tray speaks
  StatusNotifierItem); copying it into `~/.config/autostart/` starts the
  tray with the session.

## 9. Files and paths

| Path | Content | Mode |
|---|---|---|
| `$XDG_CONFIG_HOME/ataecina/` | configuration directory | `0700` |
| `…/config.json` | subscriptions, sources, thresholds, interval (no secrets) | `0600` |
| `…/keys.json` | sealed keys and wrapped master key | `0600` |
| `$XDG_RUNTIME_DIR/ataecina/snapshot.json` | latest readings (figures only) | `0600` |
| `$XDG_RUNTIME_DIR/ataecina/signals/` | signals written by hooks (Claude limits), read by the collector | `0700` / `0600` |
| `$XDG_STATE_HOME/ataecina/` | alert state, declared balances, model catalogue (`offerings.json`, §11), log offsets, log | `0700` / `0600` |

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
    { "id": "workers-ai", "source": "cloudflare-api", "account": "0123456789abcdef0123456789abcdef" },
    { "id": "zen", "source": "opencode-zen" },
    { "id": "opencode-go", "source": "opencode-console" },
    { "id": "copilot", "source": "copilot-gh", "login": "your-github-login", "monthly_allowance": 300 },
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
| `offers` | default `true`; `false` keeps `ataecina update` from fetching the published offers (§12) |
| `subscriptions[].id` | required, unique, `[a-z0-9][a-z0-9-]{0,31}` |
| `subscriptions[].source` | required, a source id of §3 |
| `subscriptions[].plan` | optional, any text, normalised to `UPPER_SNAKE_CASE`; a plan reported by the source wins |
| `subscriptions[].catalogue` | optional, any source: the models.dev provider id whose models describe the subscription (§11), lowercase letters, digits, dots and dashes; default the source's own |
| `subscriptions[].key` | only for sources that need a key; the sealed key's name, default the subscription id |
| `subscriptions[].app_server`, `app_server_after_minutes` | only for `codex-logs`; default `false` and 60 (≥ 1) |
| `subscriptions[].provider` | only for `heimdall`, where it is required |
| `subscriptions[].account` | only for `cloudflare-api`, where it is required: the 32 lowercase hexadecimal digits of the account id |
| `subscriptions[].login`, `monthly_allowance` | only for `copilot-gh`, where both are required: the GitHub login and the plan's monthly premium requests or AI credits (above 0, at most 1 000 000) |
| `subscriptions[].team` | only for `xai-api`, required: the xAI team id |
| `subscriptions[].email` | only for `cursor-admin-api`, required: the member's e-mail, sent only to Cursor |
| `subscriptions[].port` | only for `heimdall`: its admin port on loopback, 1–65535, default 4001 |

Unknown keys, and options given to a source that does not take them, are
errors. The snapshot file is internal (`version`, `taken_at`,
`interval_seconds`, `readings` with the fields of §8.2 plus
`retry_not_before`); only `status --json` is a contract.

### 9.1 Installed files

`bin/install-files` is the single description of the installed layout;
the packages (`PREFIX=/usr`), `make install-user` (`PREFIX=~/.local`, units
in `~/.local/share/systemd/user`) and the Arch `PKGBUILD` all use it.
Nothing is enabled or started by an installation.

| Path (`PREFIX=/usr`) | Content | Mode |
|---|---|---|
| `/usr/bin/ataecina` | the static binary | `0755` |
| `/usr/lib/systemd/user/ataecina-collect.service` | collector unit, `ExecStart` pointing at the installed binary | `0644` |
| `/usr/lib/systemd/user/ataecina-collect.timer` | collector timer | `0644` |
| `/usr/share/plasma/plasmoids/org.simplicer.ataecina/` | Plasma widget with compiled translations in `contents/locale/` | `0755` / `0644` |
| `/usr/share/icons/hicolor/scalable/apps/ataecina.svg` | icon | `0644` |
| `/usr/share/applications/org.simplicer.ataecina.desktop` | tray launcher (`Exec` names the installed binary) | `0644` |
| `/usr/share/doc/ataecina/` | `README.md`, `CHANGELOG.md`, `ataecina.example.json`, `LICENSE` | `0644` |

Packages: `ataecina_X.Y.Z~unstable-1_<arch>.deb`,
`ataecina-X.Y.Z~unstable-1.<arch>.rpm` and
`ataecina-X.Y.Zunstable-1-<arch>.pkg.tar.zst` for `amd64`/`x86_64` and
`arm64`/`aarch64`; they depend only on `systemd`. A pre-release suffix sorts
before its release in every format (`~unstable`, `~rc.N` for Debian and
RPM; `unstable`, `rcN` for pacman); an LTS release ships as plain `X.Y.Z`.
The same commit always gives byte-identical packages (file times are the
commit's time, `SOURCE_DATE_EPOCH`). `dist/SHA256SUMS` lists them; the
maintainer signs it for a release. `make deb` builds only the host's `.deb`;
`bin/check-packages` installs every `.deb` of the host's architecture into a
throw-away root with `fakeroot dpkg`, runs the installed binary, purges it,
and asks `apt-get --simulate` whether its dependencies resolve on the
machine; `make install-deb` then installs it with `apt`.

## 10. Constraints

- Display path: no network, no decryption, < 50 ms.
- Collector: one-shot, bounded by a global deadline (default 20 s), memory
  well under 30 MB.
- Single static binary (`CGO_ENABLED=0`); runtime dependencies: none beyond
  the session D-Bus.
- Linux only. Plasma 6 is the reference desktop; others through the tray.

## 11. Model catalogue

Besides what is left, the owner wants to know **which models each
subscription offers, which of them are free and which are new** (ADR 0015).
The bounded context `offerings` keeps that, apart from the quotas.

- **Source.** `GET https://models.dev/api.json`, the public, MIT-licensed
  catalogue of providers and models (235 providers and 8 940 models, about
  6 MB, 0.6 MB compressed, on 2026-11-08). It needs no key and none is ever
  sent. Each provider id maps to `{ "id", "name", "models": { <model id>:
  { "id", "name", "status"?, "release_date", "cost"?: { "input",
  "output", … } } } }`, prices in USD per million tokens. Coding plans are
  listed as providers of their own (`zai-coding-plan`,
  `minimax-coding-plan`, `alibaba-token-plan`, `kimi-code-plan-global`,
  `opencode-go`…) whose models cost 0: they are included in the plan.
- **When.** The collector refreshes it after collecting the readings, at
  most once a day, an hour after a failed attempt, and at once when the
  configured subscriptions need other providers. The ETag is sent back as
  `If-None-Match`, so an unchanged catalogue answers `304` with no body.
  The download has its own 25 s deadline and a 24 MiB cap after
  decompression (the only exception to the 64 KiB cap of §3, ADR 0015).
  A failure is logged and never affects the readings or the snapshot.
- **What is kept.** Only the providers of the configured subscriptions,
  without deprecated models: each model's id, name, input and output price
  (or none) and release day, in `$XDG_STATE_HOME/ataecina/offerings.json`
  (`0600`), with which provider describes each subscription. Texts are
  kept as plain text (no control or markup characters, at most 120
  characters).
- **Which provider.** By source: `zai-api` → `zai-coding-plan`,
  `minimax-api` → `minimax-coding-plan`, `alibaba-cli` →
  `alibaba-token-plan`, `alibaba-coding-cli` → `alibaba-coding-plan`, `commandcode-api` → `commandcode`,
  `openrouter-api` → `openrouter`, `vercel-api` → `vercel`,
  `opencode-zen` → `opencode`, `opencode-console` → `opencode-go`,
  `ollama-api` → `ollama-cloud`, `copilot-gh` → `github-copilot`,
  `orcarouter-api` → `orcarouter`, `cloudflare-api` →
  `cloudflare-workers-ai`, `moonshot-api` → `moonshotai`, `moonshot-cn-api`
  → `moonshotai-cn`, `kimi-code-api` → `kimi-code-plan-global`, `xai-api` → `xai`; a
  `heimdall` subscription uses the provider it
  follows (`google`, `nvidia`, `groq`, `kilo`…). Claude and Codex have
  none. The owner may name another with `"catalogue": "<provider id>"` in
  the subscription (an Alibaba Coding Plan: `alibaba-coding-plan`).
- **Free** means priced at 0 for input and output; a model without a price
  is not free (its price is unknown). **New** means released in the last 30
  days. models.dev marks no promotions, so none are claimed.
- **Displays.** `status --json` adds `offerings` to each subscription
  (§8.2). The widget's unfolded row says "7 models included" when every
  model is free (a plan), else "374 models · 16 free", then a few free
  models by name and the newest ones, always as plain text.

### 11.1 Models you can use and their intelligence (ADR 0016)

- `ataecina update` refreshes the catalogue at once and, with the sealed
  key `artificial-analysis` (a free key from https://artificialanalysis.ai,
  `ataecina keys set artificial-analysis`), the Artificial Analysis
  Intelligence Index: `GET
  https://artificialanalysis.ai/api/v2/language/models/free?page=<n>`,
  header `x-api-key`, read: `intelligence_index_version`,
  `pagination.has_more`, `data[].name`,
  `data[].evaluations.artificial_analysis_intelligence_index` (null when not
  measured; such models are skipped). At most ten pages; 401/403 is a
  rejected key. Without that key, or when it fails, the same index comes
  through OpenRouter: `GET
  https://openrouter.ai/api/v1/benchmarks?source=artificial-analysis`
  with the key of the first `openrouter-api` subscription (`Bearer`);
  rows with `source` `artificial-analysis` and a numeric
  `intelligence_index` count, each under its `display_name` and its
  `model_permaslug`; no index version; credited "via OpenRouter". Kept in
  `$XDG_STATE_HOME/ataecina/intelligence.json`
  (`0600`) until the next update. Free tier: 100 requests a day.
- A model gets the index of the ranking entry with the same key: lower case,
  no vendor prefix (`z-ai/`), no `:free`/`-free`, no release date
  (`-20260816`), no variant in parentheses
  (`(Max)`), punctuation as dashes (`glm-5-3`); the id is tried first, then
  the name; several variants keep the best index.
- `ataecina models [--json] [--all] [<subscription>]` (display path, no
  network) lists each subscription's models, best ranked first, then by
  name: `{ "data": [ { "subscription", "catalogue", "name",
  "something_left", "models": [ { "id", "name", "free", "input_price",
  "output_price" (USD per 1M tokens or null), "released_on", "intelligence"
  (or null), "available" } ] } ], "meta": { "catalogue_refreshed_at",
  "intelligence_refreshed_at", "intelligence_index_version",
  "intelligence_via" (null or `OpenRouter`), "intelligence_source",
  "intelligence_url" } }`. A model is available when
  it is free or the subscription has something left: its reading has
  figures, no window at 100 % or more and no money left at zero or below.
- The widget's unfolded row has "Models you can use": the available models
  (at most 40, then "N more"), each with its index (or "–") and its price
  per 1M tokens or "free", credited to the Artificial Analysis Intelligence
  Index with a link to artificialanalysis.ai, and "Update models", which
  runs `ataecina update`. The Keys page sets the `artificial-analysis` key.

## 12. Coding plan offers (ADR 0017)

- The maintainer publishes offers of AI coding plans in `data/offers.csv`
  of the public repository (format in `data/README.md`): `id, provider,
  plan, title, price, currency, period, url, sponsored, ends`.
- `ataecina update` fetches
  `https://codeberg.org/simplicer/ataecina/raw/branch/main/data/offers.csv`
  once, without any key (256 KiB, 15 s), unless `"offers": false` in
  `config.json`. 404 is no offer; a malformed file (another header, a link
  that is not `https`, a price or date that does not parse, more than 100
  offers) is refused whole and the kept offers stay. Kept in
  `$XDG_STATE_HOME/ataecina/offers.json` (`0600`).
- `ataecina offers [--json]` lists the offers that have not ended: `{
  "data": [ { "id", "provider", "plan", "title", "price", "currency",
  "period", "url", "sponsored", "ends" } ], "meta": { "refreshed_at" } }`.
- The widget shows them in a folded block under the warnings ("2 offers",
  the providers): provider · plan, the price per period (`month`, `year`,
  `first month` translated), the title, and "Sponsored link" for an
  affiliate link; a click opens the link in the browser.
