#!/usr/bin/env bash
# scripts/hero-update.sh — Apply a queued local Hero development update.
#
# The script is copied next to the installed Hero binary. It is intended to be
# run by the hero-update.service systemd user unit, not by the Telegram model.

set -Eeuo pipefail

UPDATE_DIR="${HERO_UPDATE_INSTALL_DIR:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)}"
SOURCE_DIR="${HERO_UPDATE_SOURCE:-}"
BUILD_SCRIPT="${HERO_UPDATE_BUILD_SCRIPT:-${SOURCE_DIR}/scripts/build_update.sh}"
HERO_BIN="${HERO_UPDATE_BIN:-${UPDATE_DIR}/hero}"
STATE_FILE="${HERO_UPDATE_STATE:-${UPDATE_DIR}/needs-update.txt}"
LOCK_FILE="${HERO_UPDATE_LOCK:-${UPDATE_DIR}/hero-update.lock}"
BACKUP_FILE="${HERO_UPDATE_BACKUP:-${UPDATE_DIR}/hero.previous}"

log() {
  printf '[hero-update] %s\n' "$*"
}

fail() {
  log "ERROR: $*" >&2
  exit 1
}

[[ -d "${UPDATE_DIR}" ]] || fail "install directory does not exist: ${UPDATE_DIR}"
[[ -n "${SOURCE_DIR}" ]] || fail "HERO_UPDATE_SOURCE is not configured"
[[ -d "${SOURCE_DIR}" ]] || fail "source directory does not exist: ${SOURCE_DIR}"
[[ -x "${BUILD_SCRIPT}" ]] || fail "build_update.sh is not executable: ${BUILD_SCRIPT}"
[[ -f "${STATE_FILE}" ]] || exit 0

exec 9>"${LOCK_FILE}"
flock -n 9 || {
  log "another update is already running"
  exit 0
}

FLAG="$(sed -n '1p' "${STATE_FILE}" 2>/dev/null || true)"
[[ "${FLAG}" == "true" ]] || exit 0

[[ -x "${HERO_BIN}" ]] || fail "installed Hero binary does not exist: ${HERO_BIN}"

NEXT_FILE="$(mktemp "${UPDATE_DIR}/.hero.next.XXXXXX")"
cleanup() {
  rm -f -- "${NEXT_FILE}"
}
trap cleanup EXIT

log "building target binary from ${SOURCE_DIR}"
HERO_UPDATE_OUTPUT="${NEXT_FILE}" "${BUILD_SCRIPT}"
[[ -x "${NEXT_FILE}" ]] || fail "build_update.sh did not create an executable"

"${NEXT_FILE}" version >/dev/null || fail "new Hero binary failed the version check"

cp -p -- "${HERO_BIN}" "${BACKUP_FILE}"
mv -f -- "${NEXT_FILE}" "${HERO_BIN}"
chmod 0755 "${HERO_BIN}"
log "installed new Hero binary; previous version saved at ${BACKUP_FILE}"

# Restart is sent through the local Telegram daemon so every registered TUI can
# re-exec itself in its existing terminal. The fallback covers a TUI that has
# no Telegram client but is running the same installed binary.
if "${HERO_BIN}" internal auto-update-restart; then
  log "restart request broadcast through the Telegram daemon"
else
  log "Telegram daemon restart request unavailable; signaling local Hero instances"
  pids=()
  for proc in /proc/[0-9]*; do
    pid="${proc##*/}"
    [[ "${pid}" == "$$" ]] && continue
    exe="$(readlink "${proc}/exe" 2>/dev/null || true)"
    case "${exe}" in
      "${HERO_BIN}"|"${HERO_BIN} (deleted)")
        pids+=("${pid}")
        ;;
    esac
  done
  for pid in "${pids[@]}"; do
    if kill -0 "${pid}" 2>/dev/null; then
      kill -USR2 "${pid}" 2>/dev/null || true
      log "restart signal sent to Hero PID ${pid}"
    fi
  done
fi

# Atomic write: the timer can never observe a partially written flag.
NEW_STATE="$(mktemp "${STATE_FILE}.XXXXXX")"
printf 'false\n' >"${NEW_STATE}"
chmod 0600 "${NEW_STATE}"
mv -f -- "${NEW_STATE}" "${STATE_FILE}"

trap - EXIT
cleanup
log "update completed"
