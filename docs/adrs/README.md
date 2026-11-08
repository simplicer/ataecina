# Architectural Decision Records

| # | Decision |
|---|---|
| [0001](0001-go-vertical-slices-in-internal.md) | Go vertical slices under `internal/` |
| [0002](0002-bounded-contexts.md) | Bounded contexts: allowances, keys, alerts (outages planned) |
| [0003](0003-local-signals-and-api-keys-never-sessions.md) | Local signals and API keys; never sessions |
| [0004](0004-sealed-keys-file-with-unlockers.md) | Sealed keys file with pluggable unlockers |
| [0005](0005-collector-and-display-separation.md) | Collector and display are separate |
| [0006](0006-outbound-network-policy.md) | Outbound network policy |
| [0007](0007-plasma-widget-first-tray-for-others.md) | Plasma widget first, a tray for other desktops |
| [0008](0008-native-packages-with-nfpm.md) | Native packages built with nfpm |
| [0009](0009-minimal-dependencies.md) | Minimal dependencies |
| [0010](0010-json-contract-and-no-http-surface.md) | JSON output contract and no HTTP surface |
| [0011](0011-source-fidelity-and-isolated-adapters.md) | Source fidelity and isolated adapters |
| [0012](0012-one-redacting-secret-type.md) | One redacting secret type shared by the contexts |
| [0013](0013-heimdall-on-loopback.md) | Heimdall is read on loopback |
| [0014](0014-keys-from-the-widget-through-a-masked-dialog.md) | Keys and balances from the widget settings, through a masked dialog |
| [0015](0015-model-catalogue-from-models-dev.md) | A daily model catalogue from models.dev |
| [0016](0016-explicit-update-command.md) | An explicit `ataecina update` for what is not refreshed on its own |
| [0017](0017-published-coding-plan-offers.md) | Coding plan offers published in the repository |
| [0018](0018-official-clis-one-at-a-time-and-kept-figures.md) | Official CLIs one at a time, and kept figures |
| [0019](0019-key-checks-for-providers-without-an-allowance.md) | Key checks for providers that publish no allowance |
| [0020](0020-peak-and-off-peak-hours.md) | Peak and off-peak hours from a compiled calendar |
| [0021](0021-published-release-downloads.md) | Published release downloads |

Each ADR has Context, Decision, Consequences and Alternatives. A superseded
ADR stays, marked `superseded by NNNN`.
