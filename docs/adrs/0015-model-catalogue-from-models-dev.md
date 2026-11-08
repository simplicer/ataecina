# ADR 0015 — A daily model catalogue from models.dev

- Status: accepted
- Date: 2026-11-08

## Context

Knowing what is left of each subscription is half the answer; the owner
also wants to know which models each subscription offers, which of them
are free, and which are new, refreshed every day. No provider API used for
quotas says that. models.dev publishes it for 235 providers, coding plans
included, as one public JSON document (`https://models.dev/api.json`,
MIT-licensed) that needs no key. It is about 6 MB once decompressed, far
above the 64 KiB cap of every provider answer (ADR 0006), and it is not a
quota: it belongs to no existing bounded context.

## Decision

- A new bounded context, `offerings`, owns the catalogue: which models a
  provider offers, their list price, which are free (priced at 0 for input
  and output) and which are new (released in the last 30 days). It knows
  nothing of quotas or keys; `allowances` knows nothing of it. The
  composition root joins them by subscription id.
- Only the collector asks for it (ADR 0005), after collecting the
  readings, at most once a day, an hour after a failure, and at once when
  the configured subscriptions need other providers. The ETag is sent
  back, so an unchanged catalogue costs a `304`. A failure is logged and
  never touches the readings.
- `https://models.dev` is compiled in like every provider host, with the
  same client: no redirects, no proxy unless enabled, one attempt. It gets
  its own limits — 24 MiB after decompression and 25 s — through
  `Endpoint.Larger`, which only raises limits and is used by this adapter
  only. No key, no `Authorization` header and no account data is ever sent
  to it (an end-to-end test checks it).
- Only the providers of the configured subscriptions are kept, without
  deprecated models, in `$XDG_STATE_HOME/ataecina/offerings.json` (`0600`).
  Every text from the catalogue is reduced to plain text (no control or
  markup characters, at most 120 characters), and the widget shows it as
  plain text only.
- `status --json` adds an `offerings` object to each subscription and
  `meta.offerings_refreshed_at`; both are additive (ADR 0010).

## Consequences

- One more host is contacted, once a day, with no secret; the README and
  SPECS §3 and §11 say so. Owners who object can drop it only by removing
  the subscriptions it describes (a switch can be added when asked).
- The collector's memory peaks while it decodes the catalogue (tens of
  MB, once a day); only the wanted providers' models are decoded.
- models.dev marks no promotions, so Ataecina claims none; "free" and
  "new" are computed from prices and release dates.

## Alternatives

- Asking each provider's model list API: one more call per provider and
  key, no prices for most, and no free flag. Rejected.
- Bundling a catalogue in the binary: stale within days. Rejected.
- Community catalogues of coding plans: unlicensed or affiliate-linked,
  mostly estimated figures. Rejected.
