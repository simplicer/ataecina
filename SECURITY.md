# Security policy

Ataecina holds API keys that can spend money. Security reports are welcome
and are handled before any other work.

## Reporting a vulnerability

Please do not open a public issue. Contact the maintainer privately through
Codeberg (<https://codeberg.org/simplicer>) and describe:

- the version (`ataecina version`) and how it was installed;
- what an attacker can do, and what they need first (local user, another
  user on the machine, network position, a malicious provider answer…);
- the smallest steps that show it, with **synthetic** keys and data only.

Never send a real key, a real `keys.json`, a snapshot, a log or a session
file, even redacted. You will get an answer within seven days; a fix is
released before the issue is disclosed, and you are credited unless you
prefer otherwise.

## Supported versions

While the major version is 0 (`0.Y.Z-unstable`), only the latest version
receives fixes. Stable releases will follow the support windows of the
Simplicer versioning policy.

## What Ataecina promises

These are the properties a report can break; the reasons live in
[docs/adrs/](docs/adrs/) and the rules in [AGENTS.md](AGENTS.md).

- **No sessions** (ADR 0003): it never reads, refreshes, stores or forwards
  OAuth tokens, session cookies or browser storage, never runs a vendor
  login and never scrapes a web console. Official CLIs keep their own
  login.
- **Sealed keys** (ADR 0004): keys enter only through `ataecina keys set`
  from stdin without echo — typed in a terminal, or in KDE's masked dialog
  from the Plasma widget's settings, piped straight to it (ADR 0014) — and
  rest only in `~/.config/ataecina/keys.json`,
  each sealed with AES-256-GCM under a random master key held by an
  unlocker (Secret Service, passphrase, systemd credentials). The key type
  redacts itself everywhere (ADR 0012).
- **Strict files**: `~/.config/ataecina` is `0700`, `config.json` and
  `keys.json` are `0600` and owned by the user, or Ataecina refuses to run.
- **Two halves** (ADR 0005): only the collector (`ataecina collect`, run by
  a systemd user unit with `NoNewPrivileges` and seccomp filters everywhere,
  and a read-only home only where unprivileged user namespaces are allowed,
  which Ubuntu forbids) unseals keys and opens network connections;
  `status`, `--json`, the tray and the Plasma widget read a snapshot of
  figures and talk to nothing but the session bus.
- **Short leash on the network** (ADR 0006): one fixed HTTPS host per
  adapter, compiled in; redirects refused; proxies ignored unless enabled;
  64 KiB answers; explicit timeouts; one attempt; `Retry-After` honoured.
  The only plain-HTTP traffic goes to Heimdall on `127.0.0.1` (ADR 0013).
- **Nothing phones home**: no telemetry, no analytics, no update checks.

## Out of scope

- An attacker who already runs code as the same user while the keyring is
  unlocked: they can ask the keyring for the master key like Ataecina does.
- Providers' own APIs and CLIs, and what they do with the keys you give
  them.
- Figures that are wrong because a provider changed an undocumented
  endpoint: that is a bug (`CONTRACT_CHANGED`), not a vulnerability.
