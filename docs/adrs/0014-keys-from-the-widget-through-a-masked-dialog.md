# ADR 0014 — Keys and balances from the widget settings, through a masked dialog

- Status: accepted
- Date: 2026-11-07

## Context

Keys enter only through `ataecina keys set <name>`, which reads stdin
without echo (ADR 0004, ADR 0012), and the display paths never hold a key
(ADR 0005). The owner asked to manage keys from the Plasma widget's
settings instead of a terminal, and to declare the OpenCode Zen balance
there too.

A QML text field would put the key in `plasmashell`'s memory, in the
settings dialog and possibly in its undo history. Plasma offers widgets no
private channel to another process: the executable engine runs a shell
command line, whose arguments every process of the user can read.

## Decision

- The widget gets a **Keys** settings page. It lists the subscriptions
  whose source needs a key, with what `ataecina status --json` says of it:
  works, no key, key rejected, or not read yet; and OpenCode Zen with its
  balance.
- **Set key…** runs, through the executable engine, exactly
  `kdialog --title Ataecina --password '<prompt>' | <ataecina> keys set '<id>'`.
  The key is typed in KDE's masked password dialog and goes through an
  anonymous pipe straight into `ataecina keys set`: never in an argument,
  the environment, `plasmashell`'s memory, QML, a log or the widget's
  settings. A cancelled dialog writes nothing, and `keys set` refuses an
  empty key, so nothing changes.
- The key name is the subscription id, the default key name (SPECS §9); a
  subscription with its own `"key"` in `config.json` is set from a terminal.
  The id is checked against `[a-z0-9][a-z0-9-]{0,31}` and every value put in
  the command line is single-quoted, quotes escaped; the prompt is fixed,
  translated text.
- **Declare balance…** (OpenCode Zen) asks the amount with
  `kdialog --inputbox` and runs `ataecina balance declare '<id>' --balance
  "$amount"`; an amount is not a secret, and the binary validates it.
  Declaring reads the spending so far with `opencode`, which pnpm installs
  off `plasmashell`'s `PATH`, so that command gets the collector unit's
  directories (`~/.local/bin`, `~/.local/share/pnpm`,
  `~/.local/share/pnpm/bin`) before the session's `PATH`.
- After either action the page starts `ataecina-collect.service` and reads
  the status again a few seconds later.
- Without `kdialog` the page shows the terminal commands instead.

## Consequences

- On an explicit click, `plasmashell` starts a process that seals a key and
  so reaches the master key through the unlocker. The widget's own code
  still never reads, holds or unseals a key; `AGENTS.md` states the
  exception.
- With the passphrase unlocker `keys set` needs a terminal; the page shows
  its error.
- The display path keeps reading only the snapshot.

## Alternatives

- A password field in the settings page: the key would live in QML and the
  settings dialog. Rejected.
- Terminal only (the previous rule): the owner found it too slow. Rejected.
- A D-Bus helper service owned by Ataecina: a resident process and more
  code for the same result. Rejected.
