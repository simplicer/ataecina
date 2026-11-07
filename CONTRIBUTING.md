# Contributing

Thank you for your interest in Ataecina. It is free software under the
GNU General Public License, version 3 or any later version
([LICENSE](LICENSE)); by sending a contribution you agree to license it
under the same terms. Issues, provider reports and discussion are
welcome. Security problems go through [SECURITY.md](SECURITY.md), never
a public issue.

## Before you start

Read [AGENTS.md](AGENTS.md): it states the rules every change follows —
architecture, security, testing, naming, versioning and commits. The
behaviour is specified in [docs/SPECS.md](docs/SPECS.md), the plan is
[docs/ROADMAP.md](docs/ROADMAP.md) and the reasons behind the rules are the
ADRs in [docs/adrs/](docs/adrs/).

## Development

Go 1.27 or later and gettext (`msgfmt`). Everything goes through the
`Makefile`:

```sh
make check      # format, vet, unit, 100 % application coverage, integration, acceptance, benchmarks, widget, vulnerabilities, build, version
make e2e        # the built binaries end to end against fake providers and fake CLIs
make fuzz       # explore every decoder further (FUZZ_TIME=30s)
make package    # .deb, .rpm and Arch packages in dist/, verified
```

`dbus-daemon` is needed for the D-Bus tests (they are skipped without it);
`qmllint` and `desktop-file-validate` are used when installed. The suite
never calls a real provider and never touches your real keys or your home.

## A change, step by step

1. Place it: bounded context (`allowances`, `keys`, `alerts`) and layer
   (`domain`, `application`, `infrastructure`); the composition root
   `cmd/ataecina` only wires.
2. Write the failing tests first, at every level that applies: unit (fake
   ports), integration (`-tags integration`), acceptance
   (`-tags acceptance`), end to end (`make e2e`).
3. Implement the smallest correct change; keep `make check` and `make e2e`
   green.
4. Update the docs it affects: SPECS, an ADR for a new decision, README,
   CHANGELOG (`[Unreleased]`).
5. Apply the version bump of your commit type with `bin/set-version`.
6. One Conventional Commit in English, `type(scope): imperative summary`
   (≤ 72 characters), whose body states What, Why, Tests, Architectural
   impact and Version impact.

## Adding a provider

Follow [AGENTS.md, "Adding a provider"](AGENTS.md#adding-a-provider-the-common-task):
document the source in SPECS §3 first, one adapter package, synthetic
fixtures and a fuzz target, registration in the composition root. Fixtures
are built from the documented shape: never paste a real answer, a real key,
an account id, an e-mail, an amount that identifies anyone, or the content
of a conversation.

## Reporting a changed provider

When a subscription shows `BROKEN` (`CONTRACT_CHANGED`), open an issue with
the provider, the source id, `ataecina version` and what changed in the
shape of the answer — field names and types only, never values.
