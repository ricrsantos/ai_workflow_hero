# ADR-C14-001 — Development Auto-update via systemd and Telegram IPC

> Development-only maintenance flow for the local Hero source checkout and
> installed binary. Status: Accepted.

| # | Decision | Status |
|---|---|---|
| ADR-076 | Development auto-update uses a host-targeted build, a systemd user timer, and Telegram IPC restart requests | Accepted |

## ADR-076: Development auto-update uses a host-targeted build, a systemd user timer, and Telegram IPC restart requests

**Context:** The Hero development binary is rebuilt frequently while several
interactive TUI instances may be connected to the local Telegram daemon.
Changing `build_dev.sh` would affect release/dev distribution behavior, while a
cron script that kills TUI processes cannot reliably restore their terminals.

**Decision:**

- `/auto-update` is a deterministic Telegram/TUI maintenance command. It
  commits the selected Hero source checkout, rejects obvious secret files, and
  atomically writes `needs-update.txt`.
- `scripts/build_update.sh` builds only `./cmd/hero` for the current
  `GOOS/GOARCH` target. It never builds the Telegram daemon, cross-compiles,
  removes `dist/`, or installs files.
- `hero-update.service` is a `systemd --user` oneshot, triggered by
  `hero-update.timer` every five minutes. The updater uses `flock`, builds a
  temporary binary, keeps one backup, atomically replaces `hero`, and clears
  the flag only after installation.
- The updater requests a restart through the existing OS-user Telegram IPC
  daemon. Registered TUIs receive an update event, stop their managed harness
  processes, and `exec` the installed binary so the existing TTY is retained.
  A `SIGUSR2` fallback covers local TUIs without a Telegram registration.
- This is a Linux/development workflow and is not part of the release or
  consumer-project upgrade contract.

**Consequences:**

- Auto-update is safe to retry after a failed build; the request flag remains
  set when the updater exits before completion.
- A binary update is independent from the Telegram daemon artifact. The local
  daemon remains managed by its existing plugin lifecycle.
- The user installs the helper and user units with
  `scripts/install_update_dev.sh` and removes them with
  `scripts/uninstall_update_dev.sh`. Uninstallation keeps the installed Hero
  binary and its `hero.previous` backup; the regular `build_dev.sh` and
  `release.sh` contracts remain unchanged.
