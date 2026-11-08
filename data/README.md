# Published data

## `offers.csv` — coding plan offers

The maintainer lists here offers of AI coding plans. `ataecina update`
(and "Update models" in the widget) fetches this file from the `main`
branch on Codeberg, without any key, unless `"offers": false` is set in
`config.json`; the widget then shows the current ones in a folded
"offers" block, and a click opens the link in the browser
([SPECS](../docs/SPECS.md) §12, [ADR 0017](../docs/adrs/0017-published-coding-plan-offers.md)).

One offer per line, with exactly this header:

```csv
id,provider,plan,title,price,currency,period,url,sponsored,ends
```

| Column | Meaning |
|---|---|
| `id` | a unique slug, `example-pro-2026-12` |
| `provider` | who sells the plan, `Example AI` |
| `plan` | the plan, `Coding Pro` |
| `title` | one short sentence about the offer (plain text) |
| `price` | a number, `5` or `4.99`; empty when it does not apply |
| `currency` | ISO 4217, `USD` |
| `period` | `month`, `year` or `first month` (translated by the widget), or any short text |
| `url` | an `https://` link to the offer |
| `sponsored` | `yes` when the link is an affiliate link (the maintainer may earn a commission); the widget then says "Sponsored link" |
| `ends` | the last day of the offer, `2026-12-31`, or empty; ended offers are not shown |

At most 100 offers. A malformed file is refused as a whole and the offers
already kept stay; quote a field that holds a comma (`"First month, then 10 USD"`).
