# ADR 0017 — Coding plan offers published in the repository

- Status: accepted
- Date: 2026-11-08

## Context

The maintainer wants to show offers of AI coding plans in the widget,
some with affiliate links, so that an owner can buy a plan from there.
Offers change at any time, independently of releases. Ataecina has no
server of its own and promises no telemetry and no update checks.

## Decision

- Offers live in `data/offers.csv` of the public repository, one per line
  with a fixed header (`id, provider, plan, title, price, currency, period,
  url, sponsored, ends`), documented in `data/README.md`.
- `ataecina update` (run by the owner, or "Update models" in the widget)
  fetches `https://codeberg.org/simplicer/ataecina/raw/branch/main/data/offers.csv`
  once, without any key, through the same hardened client (no redirects,
  256 KiB, 15 s). Nothing fetches it on a timer. A 404 (nothing published)
  is no offer; a malformed file is refused whole and the offers kept
  stay. `"offers": false` in `config.json` turns the fetch off.
- Every link must be `https`; texts are plain text; at most 100 offers;
  ended offers are not shown. They are kept in
  `$XDG_STATE_HOME/ataecina/offers.json` (`0600`) and listed by
  `ataecina offers [--json]`.
- An affiliate link is marked `sponsored` in the file and shown as
  "Sponsored link" in the widget and in the terminal. The README says that
  the maintainer may earn a commission from those links.
- The widget only opens a link when the owner clicks an offer.

## Consequences

- Codeberg sees the owner's address when `ataecina update` runs, like any
  download from it; the README says so.
- The maintainer publishes or withdraws offers with a commit, without a
  release.
- Offers never mix with the readings: they are a separate block, folded.

## Alternatives

- Offers compiled into the binary: stale between releases. Rejected.
- Fetching offers on every collection: a periodic call to a server for
  marketing. Rejected.
- A server of the maintainer's own: more to run and to trust. Rejected.
