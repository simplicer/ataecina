# ADR 0016 — An explicit `ataecina update` for what is not refreshed on its own

- Status: accepted
- Date: 2026-11-08

## Context

The collector refreshes the readings every few minutes and the model
catalogue once a day (ADR 0015). The owner also wants an intelligence
reference for each model — the Artificial Analysis Intelligence Index —
which changes only when models change, and which is served by a free API
that needs the owner's own key, allows 100 requests a day and asks for
attribution. Asking it on every collection would waste the quota for
nothing; never asking it would leave the index stale. ADR 0005 keeps
network access and key unsealing in the collector only.

## Decision

- `ataecina update` is a one-shot command the owner runs, or starts from
  the widget with "Update models": it refreshes the model catalogue at once
  (ignoring the daily schedule) and, when the key `artificial-analysis` is
  sealed, the intelligence ranking (`GET
  https://artificialanalysis.ai/api/v2/language/models/free`, header
  `x-api-key`, every page, at most ten). Like `collect`, it opens network
  connections and unseals that one key; unlike `collect`, it never runs on
  a timer and never touches the readings.
- The ranking is kept in `$XDG_STATE_HOME/ataecina/intelligence.json`
  (`0600`) until the next update. Models are matched to it by a normalised
  key (lower case, no vendor prefix, no free suffix, no variant in
  parentheses, no punctuation); several variants of a model keep the best
  index.
- `ataecina models [--json] [<subscription>]` reads the kept catalogue,
  ranking and snapshot (display path, no network): every model, best ranked
  first, priced or free, and whether it can be used with what is left
  (everything while the subscription has something left; only the free
  models when a window is used up, money ran out or the source gives no
  figure).
- Every display of the index credits Artificial Analysis and links to
  https://artificialanalysis.ai/, as its terms require.
- The same host rules as every source apply (ADR 0006), with the larger
  limits of `Endpoint.Larger` (4 MiB, 20 s); a 401/403 is reported as a
  rejected key, never retried.

## Consequences

- Without a key the catalogue still updates and the models simply have no
  index; the widget says how to add one.
- The widget gains one more command it may start on an explicit click,
  beside `keys set` and `balance declare` (ADR 0014).
- Matching by name misses models whose names differ across catalogues; an
  unmatched model shows "–" rather than a guess.

## Alternatives

- Scraping the Artificial Analysis web pages: against their terms and
  fragile. Rejected.
- Refreshing the index on every collection: wastes the daily quota and
  adds a third-party call to every run. Rejected.
- Bundling an index in the binary: stale with every model release.
  Rejected.
