#!/usr/bin/env bash
# Remove the development auto-update helper and its systemd user timer.
#
# The installed Hero binary and its hero.previous backup are deliberately kept.
# This script removes only the updater-owned helper, state, lock, and user-unit
# files. Re-running it is safe when the updater is already absent.

set -euo pipefail

INSTALL_DIR="${HERO_UPDATE_INSTALL_DIR:-/home/ricardo/installable/hero}"
USER_UNIT_DIR="${XDG_CONFIG_HOME:-${HOME}/.config}/systemd/user"
UNIT_SERVICE="${USER_UNIT_DIR}/hero-update.service"
UNIT_TIMER="${USER_UNIT_DIR}/hero-update.timer"
TIMER_WANTS_LINK="${USER_UNIT_DIR}/timers.target.wants/hero-update.timer"

log() {
  printf '[hero-update-uninstall] %s\n' "$*"
}

warn() {
  log "WARNING: $*" >&2
}

stop_systemd_units() {
  if ! command -v systemctl >/dev/null 2>&1; then
    warn "systemctl is not available; removing unit files only"
    return
  fi

  # Stop the service first so it cannot still be executing the helper when the
  # installed script is removed. `is-active` also makes an already-uninstalled
  # timer a normal no-op.
  if systemctl --user is-active --quiet hero-update.service 2>/dev/null; then
    systemctl --user stop hero-update.service >/dev/null 2>&1 || \
      warn "could not stop hero-update.service"
  fi
  if systemctl --user is-active --quiet hero-update.timer 2>/dev/null; then
    systemctl --user stop hero-update.timer >/dev/null 2>&1 || \
      warn "could not stop hero-update.timer"
  fi
  if systemctl --user is-enabled --quiet hero-update.timer 2>/dev/null; then
    systemctl --user disable hero-update.timer >/dev/null 2>&1 || \
      warn "could not disable hero-update.timer"
  fi
  systemctl --user daemon-reload >/dev/null 2>&1 || \
    warn "could not reload the systemd user manager"
}

stop_systemd_units

rm -f -- \
  "${UNIT_SERVICE}" \
  "${UNIT_TIMER}" \
  "${TIMER_WANTS_LINK}" \
  "${INSTALL_DIR}/hero-update.sh" \
  "${INSTALL_DIR}/needs-update.txt" \
  "${INSTALL_DIR}/hero-update.lock"

log "Removed Hero auto-update from ${INSTALL_DIR}"
log "Kept the Hero binary and ${INSTALL_DIR}/hero.previous"
