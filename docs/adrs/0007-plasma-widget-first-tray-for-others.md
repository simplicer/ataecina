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
  the binary, so both evolve independently.

## Alternatives

- One Qt or GTK GUI: heavy dependencies and CGO. Rejected.
- Tray only: poor experience on the reference desktop. Rejected.
