# ADR 0019 — Key checks for providers that publish no allowance

- Status: accepted
- Date: 2026-11-08

## Context

Groq, the NVIDIA API catalog and Google AI Studio give their owners an API
key and free-tier limits, but no endpoint tells what is left without
spending a request (SPECS §3, "Providers without a readable allowance").
Until now such a key had no place in Ataecina unless Heimdall served the
provider. The owner holds these keys and wants the subscriptions listed
like any other: what models they offer, which are free, and whether the key
still works.

## Decision

- A `keycheck` adapter serves the three providers, each with a fixed
  check compiled in: origin, a free, read-only `GET` that only a valid key
  is answered, the header that carries the key and the status that means a
  rejected key (SPECS §3.7):
  - Groq: `/openai/v1/models`, `Bearer`;
  - NVIDIA: the key owner's private NVCF functions, `Bearer` (the public
    model list answers any key);
  - Google AI Studio: one model of `/v1beta/models`, the key in
    `x-goog-api-key`, `400` also a rejected key.
- Nothing of the answer is read but the presence of its list. A good
  answer gives a reading `UNSUPPORTED` (no figures) whose hint says the
  key works; a refused key is `KEY_REJECTED`; the rest follows §4.3.
- The models come from the catalogue (`groq`, `nvidia`, `google` in
  models.dev, ADR 0015).
- `providerhttp.Send` returns an answer without classifying its status, so
  an adapter can apply its provider's rule before `providerhttp.Classify`.
- In the order of preference (ADR 0003) a key check comes after Heimdall:
  only Heimdall observes an allowance for these providers.

## Consequences

- One small request per subscription and collection; no tokens spent and
  nothing stored.
- The owner sees these subscriptions with their models and learns when a
  key stops working, but no meter: the reading says why.
- LLM7 stays out: no free request checks its key and models.dev lists no
  LLM7 catalogue.

## Alternatives

- **One inference request with one token** to read Groq's rate-limit
  headers: it spends the owner's free requests every few minutes and
  counts as usage. Rejected.
- **No source, catalogue only**: the key would never be used, so a revoked
  key would go unnoticed, and the subscription would need an ad-hoc source
  that contacts nothing.
- **A new outcome** (`KEY_OK`): it would change the `status --json`
  contract for a case `UNSUPPORTED` with a hint already describes.
