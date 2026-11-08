# ADR 0018 — Official CLIs one at a time, and kept figures

- Status: accepted
- Date: 2026-11-08

## Context

Some sources are read through the provider's official CLI (ADR 0003):
`opencode` for OpenCode Zen and the OpenCode Console, `gh` for Copilot,
`bl` for Alibaba, `codex app-server` for Codex. The collector asks every
subscription at once, so these processes ran side by side, and some are
heavy: each `opencode … --standalone` starts a whole OpenCode server
(measured about 160 MB for `stats`, about 600 MB for the Console limits).
The collector's systemd service peaked at 181 MB and 2 s of CPU at every
collection, every few minutes, to learn figures that had not changed.

## Decision

- **One turn.** `internal/platform/officialcli` holds a single turn: an
  official CLI starts only when no other one runs. `Command.Output` takes
  it by itself; an adapter that drives a CLI on its own (`codex
  app-server`) takes it with `officialcli.Turn` around the whole process.
  Waiting counts against the collector's deadline, not the command's own
  timeout; a source still waiting at the deadline is `UNAVAILABLE` for that
  collection only.
- **Kept figures.** `officialcli.Reuse` keeps what an adapter extracted
  (never the CLI's own output), in a `0600` file of the state directory,
  and gives it back while it is younger than a maximum age and none of the
  watched files changed. The modification times of watched files are read
  when the figures are kept, after the CLI ran, so what the CLI itself
  writes does not count. A failed or unexpected answer is never kept.
  - OpenCode Zen keeps its spend until OpenCode's database changes (OpenCode
    was used) and for at most 6 hours.
  - The OpenCode Console keeps its figures, a signed-out answer included,
    for 30 minutes.
  - `balance declare` always reads the spend live: the baseline must be
    exact.

## Consequences

- A collection with nothing new costs about 50 MB and 0.6 s of CPU; the
  OpenCode server starts only when its figures may have changed.
- Collections with several official CLIs take longer, because the CLIs
  wait for each other; the 20 s deadline bounds the wait.
- Console figures may be up to 30 minutes old, within the freshness rules
  (§4.2), and a new sign-in shows within 30 minutes.
- The state directory holds two more small files, with figures only.

## Alternatives

- **Keep the CLI's raw output.** Simpler and generic, but it would store
  OpenCode's usage statistics and workspace directory, beyond the
  documented fields (AGENTS.md), and reuse a changed contract for hours.
- **Run OpenCode's server once and talk to it over HTTP.** It would start
  the same server, add a local socket to the collector and depend on an
  undocumented API.
- **Read OpenCode's database directly.** Undocumented, and it holds prompt
  content that Ataecina must never open.
- **A cgroup memory limit on the service.** It would kill the collection
  instead of making it lighter.
