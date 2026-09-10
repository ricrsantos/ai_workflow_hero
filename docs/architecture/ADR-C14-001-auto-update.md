# ADR-C14-001 — Development Auto-update via systemd and Telegram IPC

> Development-only maintenance flow for the local Hero source checkout and
> installed binary. Status: Accepted.

| # | Decision | Status |
|---|---|---|
| ADR-076 | Development auto-update uses a host-targeted build, a systemd user timer, and resilient local TUI restart signaling | Accepted |

## ADR-076: Development auto-update uses a host-targeted build, a systemd user timer, and resilient local TUI restart signaling

**Context:** The Hero development binary is rebuilt frequently while several
interactive TUI instances may be connected to the local Telegram daemon.
Changing `build_dev.sh` would affect release/dev distribution behavior, while a
cron script that kills TUI processes cannot reliably restore their terminals.

**Decision:**

- `/auto-update` is a deterministic Telegram/TUI maintenance command. It
  commits the selected Hero source checkout, rejects obvious secret files, and
  atomically writes `needs-update.txt`.
- `scripts/build_update.sh` builds `./cmd/hero` and
  `./cmd/hero-telegram-daemon` for the current `GOOS/GOARCH` target. It never
  cross-compiles, removes `dist/`, installs files, or enables the optional
  Telegram plugin.
- `hero-update.service` is a `systemd --user` oneshot, triggered by
  `hero-update.timer` every five minutes. The updater uses `flock`, stages both
  host-targeted binaries, and atomically replaces the installed Hero binary.
  When the Telegram plugin is already installed, it atomically replaces its
  daemon and manifest in the same update transaction; an absent plugin remains
  absent.
- After staging the new artifacts, the updater stops the captured old Telegram
  daemon, then sends `SIGUSR2` directly to every matching installed Hero
  process. This is the authoritative restart path because it does not depend
  on the daemon's version, socket, or registration state. The TUI stops
  managed harnesses and `exec`s the installed binary so the existing TTY and
  process identity are retained.
- For a TUI that cannot be discovered through `/proc`, the updater keeps a
  compatibility request through the OS-user Telegram IPC daemon. That request
  has a five-second deadline and is non-blocking from the install/state
  perspective; an old protocol-v1 daemon may ignore the optional frame, so it
  must never hold the updater indefinitely. Current registrations advertise
  additive version/capability metadata, while protocol version 1 remains
  wire-compatible.
- The optional Telegram daemon has a private pidfile and is stopped when its
  plugin binary is replaced or uninstalled. Legacy daemon processes without a
  pidfile are recovered on Linux by matching their exact executable path,
  including deleted inodes. Plugin binaries and manifests are installed via
  same-directory atomic rename.
- `hero-update.service` limits one updater invocation to five minutes. Once
  the new Hero binary and, when applicable, the installed Telegram plugin have
  passed validation and been installed, the update flag is cleared even if
  restart notification is unavailable; a failed build, manifest validation, or
  installation still leaves the flag armed for retry.
- This is a Linux/development workflow and is not part of the release or
  consumer-project upgrade contract.

**Consequences:**

- Auto-update is safe to retry after a failed build; the request flag remains
  set when the updater exits before completion.
- Hero and the Telegram daemon are built from the same source revision. An
  installed Telegram plugin is updated together with Hero, including its
  manifest, while users without the optional plugin receive no automatic
  plugin installation. Plugin install/upgrade/uninstall manages the running
  daemon lifecycle so an old deleted-inode process cannot survive a
  replacement.
- The user installs the helper and user units with
  `scripts/install_update_dev.sh` and removes them with
  `scripts/uninstall_update_dev.sh`. Uninstallation keeps the installed Hero
  binary and its `hero.previous` backup. Local `build_dev.sh` and `release.sh`
  installations use the same atomic replacement and stop a running Telegram
  daemon before it is respawned from the new artifact.
