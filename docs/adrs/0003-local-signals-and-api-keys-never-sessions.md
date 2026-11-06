# ADR 0003 — Local signals and API keys; never sessions

- Status: accepted
- Date: 2026-11-06

## Context

A quota display could read the OAuth tokens stored by Claude Code and
Codex, read browser cookies or scrape web consoles, to call internal usage
endpoints. Anthropic's Claude Code terms state that
developers may not collect, store or intermediate Claude.ai credentials or
session tokens. A session token or cookie grants far more than reading usage
(billing, keys, account changes), browser stores are protected by the same
keyring that holds every other session, and consoles change without notice
and may forbid automated access.

Meanwhile, the official tools already publish what is needed locally: Claude
Code passes `rate_limits` to its status line command, and Codex writes
`rate_limits` into its session logs.

## Decision

Sources, in order of preference:

1. **Local signals written by the official tool** (Claude status line input,
   Codex session logs).
2. **Official CLIs** run with a fixed argv and no shell (Alibaba `bl`,
   `opencode stats`).
3. **Provider APIs with the owner's API key** (documented first; undocumented
   ones isolated in their adapter and verified live before merging).
4. **Heimdall**, for what it observes while serving.
5. **Declarations** by the owner (top-ups).

Never: OAuth tokens, refresh flows, session cookies, browser storage, console
scraping, vendor login flows, or imitating another client.

## Consequences

- Claude and Codex figures update when those tools are used; usage made
  elsewhere (claude.ai, ChatGPT web) appears after the next use. Displays show
  freshness and estimate rebirth after `resets_at` (SPECS §4.2).
- Some providers (OpenCode Zen balance, Gemini AI Studio keys, NVIDIA) cannot
  be read directly; they are derived, observed through Heimdall, or shown as
  unsupported.
- If a provider later offers an official usage API, it becomes a new adapter.

## Alternatives

- Reusing OAuth tokens: fresher data, but against Anthropic's terms and a
  much more powerful credential at rest. Rejected.
- Opt-in console scraping with a pasted cookie: fragile, high-privilege
  credential, often against terms. Rejected; may be reconsidered only per
  provider, through a new ADR, if no other route exists.
