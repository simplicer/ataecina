# ADR 0007 — Plasma widget first, a tray for other desktops

- Status: accepted
- Date: 2026-11-06

## Context

The maintainer uses KDE Plasma 6 and wants a native experience there; the
project should still be useful on GNOME, XFCE and other desktops when
published. A rich popup (bars, countdowns) needs a native widget; a generic
tray icon works almost everywhere but only offers an icon, a tooltip and a
menu.

## Decision

- **Plasma (first-class):** a QML plasmoid `org.simplicer.ataecina`, shipped
  in the packages under `/usr/share/plasma/plasmoids/`. It runs
  `ataecina status --json` on a timer through the Plasma 5 support
  executable engine and starts `ataecina-collect.service` for "refresh now".
  It contains no secrets logic and no network code.
- **Everywhere else:** `ataecina tray`, a StatusNotifierItem with DBusMenu
  over the session bus, which covers XFCE, Cinnamon, MATE, LXQt, Budgie,
  Waybar and GNOME with the AppIndicator extension (enabled by default on
  Ubuntu).
- A native GNOME Shell extension may follow later.

## Consequences

- On Plasma there is no extra resident process: the widget lives in
  `plasmashell`.
- The JSON contract (ADR 0010) is the only interface between the widget and
  the binary, so both evolve independently. `tests/widget` (plain `go test`)
  fails when the QML reads a name outside that contract.
- The tray starts the collector through the user's systemd manager on the
  session bus (`StartUnit`) instead of running `systemctl`, so the display
  path still runs no program and opens no socket but the session bus
  (ADR 0005).
- `qmllint` is not part of a Plasma desktop installation, so `make check`
  runs it only when it is installed and otherwise warns; the static checks of
  `tests/widget` and `msgfmt --check` always run, and the widget is checked
  by hand in Plasma before releases.

## Alternatives

- One Qt or GTK GUI: heavy dependencies and CGO. Rejected.
- Tray only: poor experience on the reference desktop. Rejected.

## Update (2026-11-07): the widget in the system tray, its popup redrawn

- The menu of `ataecina tray` is a DBusMenu that the desktop draws: labels,
  icons and separators only, with no colours, rings or layout of our own. On
  Plasma the owner found it poor and asked for a popup with one card,
  concentric rings per window, what is left in large figures and the
  countdown to the reset, in Ataecina's own colours.
- The plasmoid now declares `X-Plasma-NotificationArea` (category
  `SystemServices`): Plasma's system tray lists it among its entries and
  opens its popup from the tray icon. On Plasma the widget in the tray
  replaces `ataecina tray`; the StatusNotifierItem stays for other desktops.
- The popup is redrawn with QtQuick, Kirigami and Plasma components only (no
  new dependency), in Ataecina's palette (greens, amber, coral) rather than
  any provider's brand colours, and its figures show what is left
  (SPECS §8.3).
- The state and its wording live in `Snapshot.qml`, a `QtObject` with no
  Plasma type, so the popup loads outside `plasmashell`. While it was
  designed it was rendered offscreen with the system Qt, in light and dark
  schemes, and every QML file was compiled against the installed Plasma
  modules. The hand check in Plasma before releases stays.
