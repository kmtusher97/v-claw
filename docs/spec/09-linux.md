# 09 — Platform notes: Linux

What actually shipped for Linux, and what was deliberately left out. Read
[08-cross-platform.md](08-cross-platform.md) first — it is the plan this document
reports back against.

## Scope

Ships:

- The tray icon and menu (`fyne.io/systray` already supported Linux)
- The CLI
- `internal/power`: idle sleep, display sleep, and lid-close blocking, all via
  `systemd-logind` and the freedesktop `ScreenSaver` interface, none of it needing root
- A basic settings window, via `zenity --forms`

Deliberately not shipped, same as the roadmap called out:

- The virtual lock. There is no single shielding mechanism that works the same way
  across X11, Wayland, GNOME and KDE — Wayland in particular gives a client no way to
  cover the screen at all. A half-built lock is worse than an honest "not available
  yet", so `internal/ui.Lock` returns an error and the tray menu greys the item out.
- A privileged daemon. Unlike macOS, Linux needs none: see below.
- Diagnostics for a managed machine (the macOS report's configuration-profile check).
  Nothing analogous is wired up yet.

## No daemon, and no watchdog

The two-tier model in `02-architecture.md` still holds conceptually, but the tiers
collapse into one on Linux: `org.freedesktop.login1.Manager.Inhibit` needs no privilege
and covers lid-close blocking directly, so there is nothing left for a privileged
daemon to do. `cmd/v-clawd` stays macOS-only.

The inhibitor is a held file descriptor, returned by the `Inhibit` D-Bus call. Closing
it — including the kernel closing it automatically when this process dies — lifts the
inhibit immediately. That is the safety property `03-safety.md` asks for on macOS via a
heartbeat and a watchdog daemon; on Linux it falls out of the mechanism for free.

## Mechanism, measured

Verified against a live `systemd-logind` and a live GNOME session (Ubuntu 22.04,
Wayland) before being wired into `internal/power/power_linux.go`, per `AGENTS.md`'s
"measure before theorising":

```
$ busctl --system introspect org.freedesktop.login1 /org/freedesktop/login1
.Inhibit    method    ssss   h   -
.LidClosed  property  b      false -

$ busctl --user introspect org.freedesktop.ScreenSaver /org/freedesktop/ScreenSaver
.Inhibit    method  ss  u  -
.UnInhibit  method  u   -  -
```

| Concern | Call | Notes |
|---|---|---|
| Block idle-triggered sleep | `login1.Manager.Inhibit("idle", who, why, "block")` on the **system** bus | Always requested. Does not block an explicit `systemctl suspend` — v-claw only overrides idle and the lid, never a deliberate sleep request. |
| Block lid-close sleep | same call, `what` becomes `"idle:handle-lid-switch"` | Free. No privilege, no `pmset`-style read-back needed: the fd either exists or it doesn't. |
| Keep the display on | `ScreenSaver.Inhibit(who, why)` on the **session** bus, `ScreenSaver.UnInhibit(cookie)` to release | Screen blanking is a desktop-session concern, not logind's, so it needs the session bus. If no session bus is reachable (a headless context), this piece degrades and the other two still work. |
| AC power | `/sys/class/power_supply/*/type` == `Mains`, then its `online` file | No running service required. A machine with no `Mains` node at all (a desktop, or a container) is treated as always plugged in. |
| Lid present / closed | `/proc/acpi/button/lid/*/state` existence for "has a lid", `login1.Manager.LidClosed` for its value | The property is read from logind rather than the ACPI file directly, so it can never disagree with the Inhibit call that already depends on the same source. |
| Idle time | not implemented | Nothing needs it without the virtual lock. X11 has `XScreenSaverQueryInfo`; Wayland has the idle-notify protocol; picking between them is deferred until there is a caller. |

This was confirmed live, not just read from documentation: acquiring the inhibitor
showed up in `systemd-inhibit --list` under the name `v-claw`, and releasing it made the
entry disappear.

## Settings window

`zenity --forms` rather than a GTK build dependency. There is no `--combo-default` flag
in stable zenity, so the current value is put first in each combo's value list instead —
zenity pre-selects the first entry, which produces the same visible effect. The dialog
cannot be pushed to once open (unlike the darwin helper, which is a long-lived process
driven over stdin/stdout), so `internal/ui.(*UI).Push` on Linux is a no-op: the window
shows state as of the moment it was opened.

## Install

No daemon means no privilege, anywhere. `make install` on Linux copies the two binaries
to `~/.local/bin` and writes an XDG autostart entry
(`~/.config/autostart/v-claw.desktop`) — the freedesktop equivalent of the macOS
`LaunchAgent`, and readable by GNOME, KDE, and XFCE alike without depending on systemd
user units being enabled to linger.
