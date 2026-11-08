# ADR 0020 — Peak and off-peak hours from a compiled calendar

- Status: accepted
- Date: 2026-11-08

## Context

Some plans charge the same use differently by the hour: Z.ai's GLM Coding
Plan takes half the credits outside 14:00–18:00 UTC+8 on weekdays, the
Model Studio Token Plan discounts the night (22:00–08:00 UTC+8), and
Claude's session window drains faster on weekday mornings, Pacific time.
The owner wants to see, on every subscription, when using it costs less,
and which subscriptions have nothing left right now. No provider exposes
these hours through an API: they are published in plan pages and
announcements, and they change with promotions.

## Decision

- The domain gets `PeakHours` (a time zone, the days, one span per day,
  what off-peak saves) and `RatePeriod` (peak or off-peak now, when that
  ends, the current or next peak, the saving). It is pure: the zone and the
  instant are passed in.
- The source catalogue carries the peak hours of `zai-api`, `alibaba-cli`
  and `claude-statusline` (SPECS §13). Claude's zone comes from the system
  time zone database; without it Claude has none rather than wrong ones.
- The display path computes where now falls, from the clock, when it
  writes: `status --json` adds `rate_period` to each subscription (`null`
  without published hours; additive to the contract, ADR 0010), the
  terminal adds "off-peak until 07:00", the widget tints off-peak rows and
  says until when. Nothing is fetched or stored; the collector is not
  involved.
- The widget also shows a subscription's name in red while nothing is left
  to use (a window at 100 % or no money left), from the figures it already
  has.

## Consequences

- The hours are as current as the installed version: a provider's change
  needs a release with SPECS §13 updated. Each row records when and where
  it was checked.
- Peak hours show even when the subscription's reading failed, since they
  belong to the plan, not to the reading.
- The tray does not show them yet.

## Alternatives

- **Hours in `config.json`**: flexible, but every owner would have to look
  them up and keep them current; the published rules are the same for
  everyone. A later version may let the owner override them.
- **Fetching them from a published file** (like the offers, ADR 0017):
  hours would update without a release, at the cost of another fetch and
  of trusting a file for figures that drive decisions. Not worth it while
  only three plans publish hours.
- **Computing it in the collector and storing it in the snapshot**: the
  period would be stale between collections and when the timer stops.
